package app

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(dsn string) (*MySQLRepository, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	repo := &MySQLRepository{db: db}
	if err := repo.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return repo, nil
}

func (r *MySQLRepository) Close() error {
	return r.db.Close()
}

func (r *MySQLRepository) init() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS class_groups (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			subject VARCHAR(255) NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS students (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			Name VARCHAR(255) NOT NULL,
			groupe BIGINT NOT NULL,
			CONSTRAINT fk_students_group FOREIGN KEY (groupe) REFERENCES class_groups(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS lessons (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			groupe BIGINT NOT NULL,
			subject VARCHAR(255) NOT NULL,
			theme TEXT NOT NULL,
			date DATE NOT NULL,
			term VARCHAR(255) NOT NULL,
			UNIQUE KEY uniq_group_date (groupe, date),
			CONSTRAINT fk_lessons_group FOREIGN KEY (groupe) REFERENCES class_groups(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS scores (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			student BIGINT NOT NULL,
			lesson BIGINT NOT NULL,
			value INT NULL,
			UNIQUE KEY uniq_student_lesson (student, lesson),
			CONSTRAINT fk_scores_student FOREIGN KEY (student) REFERENCES students(id) ON DELETE CASCADE,
			CONSTRAINT fk_scores_lesson FOREIGN KEY (lesson) REFERENCES lessons(id) ON DELETE CASCADE
		)`,
	}

	for _, stmt := range statements {
		if _, err := r.db.Exec(stmt); err != nil {
			return fmt.Errorf("init mysql schema: %w", err)
		}
	}

	return r.db.Ping()
}

func (r *MySQLRepository) ListGroups() (Store, error) {
	groups, err := r.listGroups(r.db)
	if err != nil {
		return Store{}, err
	}
	return Store{Groups: groups}, nil
}

func (r *MySQLRepository) GetGroup(id string) (Group, error) {
	groupID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return Group{}, errors.New("group not found")
	}

	groups, err := r.listGroups(r.db, groupID)
	if err != nil {
		return Group{}, err
	}
	if len(groups) == 0 {
		return Group{}, errors.New("group not found")
	}
	return groups[0], nil
}

func (r *MySQLRepository) DeleteGroup(id string) error {
	groupID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return errors.New("group not found")
	}

	result, err := r.db.Exec(`DELETE FROM class_groups WHERE id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted group count: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("group not found")
	}

	return nil
}

func (r *MySQLRepository) CreateUser(email, passwordHash string) (User, error) {
	result, err := r.db.Exec(`INSERT INTO users (email, password_hash) VALUES (?, ?)`, email, passwordHash)
	if err != nil {
		if isDuplicateEntry(err) {
			return User{}, errors.New("email already exists")
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("read user id: %w", err)
	}

	return User{
		ID:    strconv.FormatInt(userID, 10),
		Email: email,
	}, nil
}

func (r *MySQLRepository) AuthenticateUser(email string) (User, string, error) {
	var (
		userID       int64
		passwordHash string
	)

	if err := r.db.QueryRow(`SELECT id, password_hash FROM users WHERE email = ?`, email).Scan(&userID, &passwordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, "", errors.New("user not found")
		}
		return User{}, "", fmt.Errorf("load user: %w", err)
	}

	return User{
		ID:    strconv.FormatInt(userID, 10),
		Email: email,
	}, passwordHash, nil
}

func (r *MySQLRepository) CreateGroup(name, subject string, students []Student) (Group, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return Group{}, fmt.Errorf("begin create group: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`INSERT INTO class_groups (name, subject) VALUES (?, ?)`, name, subject)
	if err != nil {
		return Group{}, fmt.Errorf("insert group: %w", err)
	}

	groupID, err := result.LastInsertId()
	if err != nil {
		return Group{}, fmt.Errorf("read group id: %w", err)
	}

	insertStudent, err := tx.Prepare(`INSERT INTO students (Name, groupe) VALUES (?, ?)`)
	if err != nil {
		return Group{}, fmt.Errorf("prepare student insert: %w", err)
	}
	defer insertStudent.Close()

	groupStudents := make([]Student, 0, len(students))
	for _, student := range students {
		result, err := insertStudent.Exec(student.Name, groupID)
		if err != nil {
			return Group{}, fmt.Errorf("insert student: %w", err)
		}
		studentID, err := result.LastInsertId()
		if err != nil {
			return Group{}, fmt.Errorf("read student id: %w", err)
		}
		groupStudents = append(groupStudents, Student{
			ID:   strconv.FormatInt(studentID, 10),
			Name: student.Name,
		})
	}

	if err := tx.Commit(); err != nil {
		return Group{}, fmt.Errorf("commit create group: %w", err)
	}

	return Group{
		ID:       strconv.FormatInt(groupID, 10),
		Name:     name,
		Subject:  subject,
		Students: groupStudents,
		Lessons:  []Lesson{},
	}, nil
}

func (r *MySQLRepository) CreateLesson(groupID, date string) (Lesson, error) {
	group, err := r.GetGroup(groupID)
	if err != nil {
		return Lesson{}, err
	}

	groupIDNum, _ := strconv.ParseInt(groupID, 10, 64)
	tx, err := r.db.Begin()
	if err != nil {
		return Lesson{}, fmt.Errorf("begin create lesson: %w", err)
	}
	defer tx.Rollback()

	var lessonID int64
	err = tx.QueryRow(`SELECT id FROM lessons WHERE groupe = ? AND date = ?`, groupIDNum, date).Scan(&lessonID)
	switch {
	case err == nil:
		records, err := r.lessonRecords(tx, lessonID)
		if err != nil {
			return Lesson{}, err
		}
		var theme, term string
		if err := tx.QueryRow(`SELECT theme, term FROM lessons WHERE id = ?`, lessonID).Scan(&theme, &term); err != nil {
			return Lesson{}, fmt.Errorf("load lesson: %w", err)
		}
		return Lesson{
			ID:      strconv.FormatInt(lessonID, 10),
			Date:    date,
			Theme:   theme,
			Term:    term,
			Records: recordsWithStudents(group.Students, records),
		}, nil
	case !errors.Is(err, sql.ErrNoRows):
		return Lesson{}, fmt.Errorf("lookup lesson: %w", err)
	}

	result, err := tx.Exec(
		`INSERT INTO lessons (groupe, subject, theme, date, term) VALUES (?, ?, '', ?, '')`,
		groupIDNum,
		group.Subject,
		date,
	)
	if err != nil {
		return Lesson{}, fmt.Errorf("insert lesson: %w", err)
	}

	lessonID, err = result.LastInsertId()
	if err != nil {
		return Lesson{}, fmt.Errorf("read lesson id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Lesson{}, fmt.Errorf("commit create lesson: %w", err)
	}

	return Lesson{
		ID:      strconv.FormatInt(lessonID, 10),
		Date:    date,
		Theme:   "",
		Term:    "",
		Records: recordsWithStudents(group.Students, nil),
	}, nil
}

func (r *MySQLRepository) UpdateLessonRecords(groupID, lessonDate string, lesson Lesson) (Lesson, error) {
	group, err := r.GetGroup(groupID)
	if err != nil {
		return Lesson{}, err
	}

	groupIDNum, _ := strconv.ParseInt(groupID, 10, 64)
	tx, err := r.db.Begin()
	if err != nil {
		return Lesson{}, fmt.Errorf("begin update lesson: %w", err)
	}
	defer tx.Rollback()

	var lessonID int64
	if err := tx.QueryRow(`SELECT id FROM lessons WHERE groupe = ? AND date = ?`, groupIDNum, lessonDate).Scan(&lessonID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Lesson{}, errors.New("lesson not found")
		}
		return Lesson{}, fmt.Errorf("lookup lesson: %w", err)
	}

	allowed := make(map[string]struct{}, len(group.Students))
	for _, student := range group.Students {
		allowed[student.ID] = struct{}{}
	}
	for studentID := range lesson.Records {
		if _, ok := allowed[studentID]; !ok {
			return Lesson{}, errors.New("unknown student")
		}
	}

	if _, err := tx.Exec(
		`UPDATE lessons SET theme = ?, term = ?, subject = ? WHERE id = ?`,
		strings.TrimSpace(lesson.Theme),
		strings.TrimSpace(lesson.Term),
		group.Subject,
		lessonID,
	); err != nil {
		return Lesson{}, fmt.Errorf("update lesson: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM scores WHERE lesson = ?`, lessonID); err != nil {
		return Lesson{}, fmt.Errorf("delete scores: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO scores (student, lesson, value) VALUES (?, ?, ?)`)
	if err != nil {
		return Lesson{}, fmt.Errorf("prepare score insert: %w", err)
	}
	defer stmt.Close()

	records := make(map[string]AttendanceRecord, len(group.Students))
	for _, student := range group.Students {
		record := lesson.Records[student.ID]
		records[student.ID] = AttendanceRecord{
			Present: record.Present,
			Score:   cloneScore(record.Score),
		}
		if !record.Present {
			continue
		}

		studentIDNum, err := strconv.ParseInt(student.ID, 10, 64)
		if err != nil {
			return Lesson{}, errors.New("unknown student")
		}
		if _, err := stmt.Exec(studentIDNum, lessonID, record.Score); err != nil {
			return Lesson{}, fmt.Errorf("insert score: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Lesson{}, fmt.Errorf("commit update lesson: %w", err)
	}

	return Lesson{
		ID:      strconv.FormatInt(lessonID, 10),
		Date:    lessonDate,
		Theme:   strings.TrimSpace(lesson.Theme),
		Term:    strings.TrimSpace(lesson.Term),
		Records: records,
	}, nil
}

func (r *MySQLRepository) ImportStore(store Store) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin import: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range []string{
		`DELETE FROM scores`,
		`DELETE FROM lessons`,
		`DELETE FROM students`,
		`DELETE FROM class_groups`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("clear import target: %w", err)
		}
	}

	for _, group := range store.Groups {
		result, err := tx.Exec(`INSERT INTO class_groups (name, subject) VALUES (?, ?)`, group.Name, group.Subject)
		if err != nil {
			return fmt.Errorf("import group: %w", err)
		}
		groupID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("read imported group id: %w", err)
		}

		studentIDMap := make(map[string]int64, len(group.Students))
		for _, student := range group.Students {
			result, err := tx.Exec(`INSERT INTO students (Name, groupe) VALUES (?, ?)`, student.Name, groupID)
			if err != nil {
				return fmt.Errorf("import student: %w", err)
			}
			studentID, err := result.LastInsertId()
			if err != nil {
				return fmt.Errorf("read imported student id: %w", err)
			}
			studentIDMap[student.ID] = studentID
		}

		sort.Slice(group.Lessons, func(i, j int) bool {
			return group.Lessons[i].Date < group.Lessons[j].Date
		})

		for _, lesson := range group.Lessons {
			result, err := tx.Exec(
				`INSERT INTO lessons (groupe, subject, theme, date, term) VALUES (?, ?, ?, ?, ?)`,
				groupID,
				group.Subject,
				lesson.Theme,
				lesson.Date,
				lesson.Term,
			)
			if err != nil {
				return fmt.Errorf("import lesson: %w", err)
			}
			lessonID, err := result.LastInsertId()
			if err != nil {
				return fmt.Errorf("read imported lesson id: %w", err)
			}

			for studentID, record := range lesson.Records {
				if !record.Present {
					continue
				}
				targetStudentID, ok := studentIDMap[studentID]
				if !ok {
					return errors.New("unknown student")
				}
				if _, err := tx.Exec(
					`INSERT INTO scores (student, lesson, value) VALUES (?, ?, ?)`,
					targetStudentID,
					lessonID,
					record.Score,
				); err != nil {
					return fmt.Errorf("import score: %w", err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit import: %w", err)
	}
	return nil
}

type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func (r *MySQLRepository) listGroups(q queryer, ids ...int64) ([]Group, error) {
	query := `
		SELECT g.id, g.name, g.subject,
		       s.id, s.Name,
		       l.id, l.date, l.theme, l.term,
		       sc.student, sc.value
		FROM class_groups g
		LEFT JOIN students s ON s.groupe = g.id
		LEFT JOIN lessons l ON l.groupe = g.id
		LEFT JOIN scores sc ON sc.lesson = l.id AND sc.student = s.id
	`
	args := []any{}
	if len(ids) > 0 {
		query += ` WHERE g.id = ?`
		args = append(args, ids[0])
	}
	query += ` ORDER BY g.id, s.id, l.date`

	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query groups: %w", err)
	}
	defer rows.Close()

	type lessonKey struct {
		groupID  int64
		lessonID int64
	}

	groupOrder := []int64{}
	groups := map[int64]*Group{}
	studentsSeen := map[int64]map[int64]struct{}{}
	lessonsSeen := map[int64]map[int64]*Lesson{}

	for rows.Next() {
		var (
			groupID      int64
			groupName    string
			groupSubject string
			studentID    sql.NullInt64
			studentName  sql.NullString
			lessonID     sql.NullInt64
			lessonDate   sql.NullString
			lessonTheme  sql.NullString
			lessonTerm   sql.NullString
			scoreStudent sql.NullInt64
			scoreValue   sql.NullInt64
		)

		if err := rows.Scan(
			&groupID,
			&groupName,
			&groupSubject,
			&studentID,
			&studentName,
			&lessonID,
			&lessonDate,
			&lessonTheme,
			&lessonTerm,
			&scoreStudent,
			&scoreValue,
		); err != nil {
			return nil, fmt.Errorf("scan group row: %w", err)
		}

		group, ok := groups[groupID]
		if !ok {
			group = &Group{
				ID:       strconv.FormatInt(groupID, 10),
				Name:     groupName,
				Subject:  groupSubject,
				Students: []Student{},
				Lessons:  []Lesson{},
			}
			groups[groupID] = group
			groupOrder = append(groupOrder, groupID)
			studentsSeen[groupID] = map[int64]struct{}{}
			lessonsSeen[groupID] = map[int64]*Lesson{}
		}

		if studentID.Valid {
			if _, ok := studentsSeen[groupID][studentID.Int64]; !ok {
				group.Students = append(group.Students, Student{
					ID:   strconv.FormatInt(studentID.Int64, 10),
					Name: studentName.String,
				})
				studentsSeen[groupID][studentID.Int64] = struct{}{}
			}
		}

		if lessonID.Valid {
			lesson, ok := lessonsSeen[groupID][lessonID.Int64]
			if !ok {
				lesson = &Lesson{
					ID:      strconv.FormatInt(lessonID.Int64, 10),
					Date:    normalizeLessonDate(lessonDate.String),
					Theme:   lessonTheme.String,
					Term:    lessonTerm.String,
					Records: map[string]AttendanceRecord{},
				}
				lessonsSeen[groupID][lessonID.Int64] = lesson
				group.Lessons = append(group.Lessons, *lesson)
				lesson = &group.Lessons[len(group.Lessons)-1]
				lessonsSeen[groupID][lessonID.Int64] = lesson
			}

			if studentID.Valid {
				key := strconv.FormatInt(studentID.Int64, 10)
				record := AttendanceRecord{}
				if scoreStudent.Valid && scoreStudent.Int64 == studentID.Int64 {
					record.Present = true
					if scoreValue.Valid {
						value := int(scoreValue.Int64)
						record.Score = &value
					}
				}
				lesson.Records[key] = record
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group rows: %w", err)
	}

	result := make([]Group, 0, len(groupOrder))
	for _, groupID := range groupOrder {
		group := groups[groupID]
		for li := range group.Lessons {
			group.Lessons[li].Records = recordsWithStudents(group.Students, group.Lessons[li].Records)
		}
		result = append(result, *group)
	}
	return result, nil
}

func normalizeLessonDate(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) >= len("2006-01-02") {
		return trimmed[:len("2006-01-02")]
	}
	return trimmed
}

func (r *MySQLRepository) lessonRecords(q queryer, lessonID int64) (map[string]AttendanceRecord, error) {
	rows, err := q.Query(`SELECT student, value FROM scores WHERE lesson = ?`, lessonID)
	if err != nil {
		return nil, fmt.Errorf("query lesson records: %w", err)
	}
	defer rows.Close()

	records := map[string]AttendanceRecord{}
	for rows.Next() {
		var studentID int64
		var score sql.NullInt64
		if err := rows.Scan(&studentID, &score); err != nil {
			return nil, fmt.Errorf("scan lesson record: %w", err)
		}
		record := AttendanceRecord{Present: true}
		if score.Valid {
			value := int(score.Int64)
			record.Score = &value
		}
		records[strconv.FormatInt(studentID, 10)] = record
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lesson records: %w", err)
	}

	return records, nil
}

func recordsWithStudents(students []Student, existing map[string]AttendanceRecord) map[string]AttendanceRecord {
	records := make(map[string]AttendanceRecord, len(students))
	for _, student := range students {
		record := AttendanceRecord{}
		if existing != nil {
			if saved, ok := existing[student.ID]; ok {
				record.Present = saved.Present
				record.Score = cloneScore(saved.Score)
			}
		}
		records[student.ID] = record
	}
	return records
}

func isDuplicateEntry(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}
	return false
}
