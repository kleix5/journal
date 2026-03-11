package app

import (
	"errors"
	"slices"
	"strings"
)

type MemoryRepository struct {
	store Store
	users []memoryUser
}

type memoryUser struct {
	user         User
	passwordHash string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		store: Store{Groups: []Group{}},
	}
}

func (r *MemoryRepository) Close() error {
	return nil
}

func (r *MemoryRepository) ListGroups() (Store, error) {
	return cloneStore(r.store), nil
}

func (r *MemoryRepository) CreateGroup(name, subject string, students []Student) (Group, error) {
	group := Group{
		ID:       newID(),
		Name:     name,
		Subject:  subject,
		Students: cloneStudents(students),
		Lessons:  []Lesson{},
	}
	r.store.Groups = append(r.store.Groups, group)
	return cloneGroup(group), nil
}

func (r *MemoryRepository) GetGroup(id string) (Group, error) {
	for _, group := range r.store.Groups {
		if group.ID == id {
			return cloneGroup(group), nil
		}
	}
	return Group{}, errors.New("group not found")
}

func (r *MemoryRepository) CreateLesson(groupID, date string) (Lesson, error) {
	for gi := range r.store.Groups {
		group := &r.store.Groups[gi]
		if group.ID != groupID {
			continue
		}

		for _, lesson := range group.Lessons {
			if lesson.Date == date {
				return cloneLesson(lesson), nil
			}
		}

		lesson := Lesson{
			ID:      newID(),
			Date:    date,
			Theme:   "",
			Term:    "",
			Records: map[string]AttendanceRecord{},
		}
		for _, student := range group.Students {
			lesson.Records[student.ID] = AttendanceRecord{}
		}

		group.Lessons = append(group.Lessons, lesson)
		return cloneLesson(lesson), nil
	}

	return Lesson{}, errors.New("group not found")
}

func (r *MemoryRepository) UpdateLessonRecords(groupID, lessonDate string, lesson Lesson) (Lesson, error) {
	for gi := range r.store.Groups {
		group := &r.store.Groups[gi]
		if group.ID != groupID {
			continue
		}

		for li := range group.Lessons {
			if group.Lessons[li].Date != lessonDate {
				continue
			}

			allowed := make(map[string]struct{}, len(group.Students))
			for _, student := range group.Students {
				allowed[student.ID] = struct{}{}
			}

			records := make(map[string]AttendanceRecord, len(group.Students))
			for _, student := range group.Students {
				record := lesson.Records[student.ID]
				records[student.ID] = record
			}
			for studentID := range lesson.Records {
				if _, ok := allowed[studentID]; !ok {
					return Lesson{}, errors.New("unknown student")
				}
			}

			group.Lessons[li].Theme = strings.TrimSpace(lesson.Theme)
			group.Lessons[li].Term = strings.TrimSpace(lesson.Term)
			group.Lessons[li].Records = records
			return cloneLesson(group.Lessons[li]), nil
		}
		return Lesson{}, errors.New("lesson not found")
	}

	return Lesson{}, errors.New("group not found")
}

func (r *MemoryRepository) ImportStore(store Store) error {
	r.store = cloneStore(store)
	return nil
}

func (r *MemoryRepository) CreateUser(email, passwordHash string) (User, error) {
	for _, user := range r.users {
		if strings.EqualFold(user.user.Email, email) {
			return User{}, errors.New("email already exists")
		}
	}

	user := User{
		ID:    newID(),
		Email: email,
	}
	r.users = append(r.users, memoryUser{
		user:         user,
		passwordHash: passwordHash,
	})
	return user, nil
}

func (r *MemoryRepository) AuthenticateUser(email string) (User, string, error) {
	for _, user := range r.users {
		if strings.EqualFold(user.user.Email, email) {
			return user.user, user.passwordHash, nil
		}
	}
	return User{}, "", errors.New("user not found")
}

func cloneStore(store Store) Store {
	groups := make([]Group, 0, len(store.Groups))
	for _, group := range store.Groups {
		groups = append(groups, cloneGroup(group))
	}
	return Store{Groups: groups}
}

func cloneGroup(group Group) Group {
	return Group{
		ID:       group.ID,
		Name:     group.Name,
		Subject:  group.Subject,
		Students: cloneStudents(group.Students),
		Lessons:  cloneLessons(group.Lessons),
	}
}

func cloneStudents(students []Student) []Student {
	return slices.Clone(students)
}

func cloneLessons(lessons []Lesson) []Lesson {
	cloned := make([]Lesson, 0, len(lessons))
	for _, lesson := range lessons {
		cloned = append(cloned, cloneLesson(lesson))
	}
	return cloned
}

func cloneLesson(lesson Lesson) Lesson {
	records := make(map[string]AttendanceRecord, len(lesson.Records))
	for studentID, record := range lesson.Records {
		records[studentID] = AttendanceRecord{
			Present: record.Present,
			Score:   cloneScore(record.Score),
		}
	}
	return Lesson{
		ID:      lesson.ID,
		Date:    lesson.Date,
		Theme:   lesson.Theme,
		Term:    lesson.Term,
		Records: records,
	}
}

func cloneScore(score *int) *int {
	if score == nil {
		return nil
	}
	value := *score
	return &value
}
