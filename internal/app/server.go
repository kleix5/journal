package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Student struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AttendanceRecord struct {
	Present bool `json:"present"`
	Score   *int `json:"score,omitempty"`
}

type Lesson struct {
	ID      string                      `json:"id,omitempty"`
	Date    string                      `json:"date"`
	Theme   string                      `json:"theme"`
	Term    string                      `json:"term"`
	Records map[string]AttendanceRecord `json:"records"`
}

type Group struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Subject  string    `json:"subject"`
	Students []Student `json:"students"`
	Lessons  []Lesson  `json:"lessons"`
}

type Store struct {
	Groups []Group `json:"groups"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Repository interface {
	ListGroups() (Store, error)
	CreateGroup(name, subject string, students []Student) (Group, error)
	GetGroup(id string) (Group, error)
	CreateLesson(groupID, date string) (Lesson, error)
	UpdateLessonRecords(groupID, lessonDate string, lesson Lesson) (Lesson, error)
	ImportStore(store Store) error
	CreateUser(email, passwordHash string) (User, error)
	AuthenticateUser(email string) (User, string, error)
	Close() error
}

type Server struct {
	mux   *http.ServeMux
	repo  Repository
	mu    sync.Mutex
	logs  []LogEntry
	logMu sync.Mutex
}

func NewServer(_ string) (*Server, error) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if dsn == "" {
		return nil, errors.New("MYSQL_DSN is required")
	}

	repo, err := NewMySQLRepository(dsn)
	if err != nil {
		return nil, err
	}

	return NewServerWithRepository(repo), nil
}

func NewServerWithRepository(repo Repository) *Server {
	s := &Server{
		mux:  http.NewServeMux(),
		repo: repo,
	}
	s.routes()
	return s
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.logMiddleware(s.mux))
}

func (s *Server) Close() error {
	if s.repo == nil {
		return nil
	}
	return s.repo.Close()
}

func (s *Server) routes() {
	s.mux.Handle("/", http.FileServer(http.Dir("web")))
	s.mux.HandleFunc("/api/login", s.handleLogin)
	s.mux.HandleFunc("/api/register", s.handleRegister)
	s.mux.HandleFunc("/api/groups", s.handleGroups)
	s.mux.HandleFunc("/api/groups/", s.handleGroupRoutes)
	s.mux.HandleFunc("/api/journal/export", s.handleExport)
	s.mux.HandleFunc("/api/journal/import", s.handleImport)
	s.mux.HandleFunc("/api/logs", s.handleLogs)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		writeError(w, http.StatusBadRequest, "email must be valid")
		return
	}
	if len(password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure password")
		return
	}

	user, err := s.repo.CreateUser(email, string(passwordHash))
	if err != nil {
		if err.Error() == "email already exists" {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to register user")
		return
	}

	s.logInfo("server", "user.register", map[string]string{"email": email})
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)
	if email == "" || password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, passwordHash, err := s.repo.AuthenticateUser(email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	s.logInfo("server", "user.login", map[string]string{"email": email})
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		store, err := s.repo.ListGroups()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load groups")
			return
		}
		writeJSON(w, http.StatusOK, store)
	case http.MethodPost:
		var req struct {
			Name     string   `json:"name"`
			Subject  string   `json:"subject"`
			Students []string `json:"students"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		name := strings.TrimSpace(req.Name)
		subject := strings.TrimSpace(req.Subject)
		if name == "" {
			writeError(w, http.StatusBadRequest, "group name is required")
			return
		}
		if subject == "" {
			writeError(w, http.StatusBadRequest, "subject is required")
			return
		}

		students := normalizeStudents(req.Students)
		if len(students) == 0 {
			writeError(w, http.StatusBadRequest, "at least one student is required")
			return
		}

		group, err := s.repo.CreateGroup(name, subject, students)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save group")
			return
		}

		s.logInfo("server", "group.create", map[string]string{
			"groupId": group.ID,
			"name":    group.Name,
			"subject": group.Subject,
			"count":   fmt.Sprintf("%d", len(group.Students)),
		})
		writeJSON(w, http.StatusCreated, group)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleGroupRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/groups/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	groupID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		s.handleGroup(w, groupID)
		return
	}

	switch parts[1] {
	case "lessons":
		s.handleLessons(w, r, groupID, parts[2:])
	default:
		writeError(w, http.StatusNotFound, "route not found")
	}
}

func (s *Server) handleGroup(w http.ResponseWriter, groupID string) {
	group, err := s.repo.GetGroup(groupID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, group)
}

func (s *Server) handleLessons(w http.ResponseWriter, r *http.Request, groupID string, rest []string) {
	if len(rest) == 0 {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		s.createLesson(w, r, groupID)
		return
	}

	if len(rest) == 2 && rest[1] == "records" {
		if r.Method != http.MethodPut {
			writeMethodNotAllowed(w)
			return
		}
		s.updateLessonRecords(w, r, groupID, rest[0])
		return
	}

	writeError(w, http.StatusNotFound, "route not found")
}

func (s *Server) createLesson(w http.ResponseWriter, r *http.Request, groupID string) {
	var req struct {
		Date string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	lessonDate := time.Now().Format("2006-01-02")
	if trimmed := strings.TrimSpace(req.Date); trimmed != "" {
		if _, err := time.Parse("2006-01-02", trimmed); err != nil {
			writeError(w, http.StatusBadRequest, "date must be in YYYY-MM-DD format")
			return
		}
		lessonDate = trimmed
	}

	lesson, err := s.repo.CreateLesson(groupID, lessonDate)
	if err != nil {
		if err.Error() == "group not found" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save lesson")
		return
	}

	s.logInfo("server", "lesson.create", map[string]string{
		"groupId": groupID,
		"date":    lessonDate,
	})
	writeJSON(w, http.StatusCreated, lesson)
}

func (s *Server) updateLessonRecords(w http.ResponseWriter, r *http.Request, groupID, lessonDate string) {
	var req struct {
		Theme   string                      `json:"theme"`
		Term    string                      `json:"term"`
		Comment string                      `json:"comment"`
		Records map[string]AttendanceRecord `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, record := range req.Records {
		if record.Score != nil && (*record.Score < 1 || *record.Score > 5) {
			writeError(w, http.StatusBadRequest, "score must be between 1 and 5")
			return
		}
	}

	theme := strings.TrimSpace(req.Theme)
	if theme == "" {
		theme = strings.TrimSpace(req.Comment)
	}

	lesson, err := s.repo.UpdateLessonRecords(groupID, lessonDate, Lesson{
		Date:    lessonDate,
		Theme:   theme,
		Term:    strings.TrimSpace(req.Term),
		Records: req.Records,
	})
	if err != nil {
		switch err.Error() {
		case "group not found", "lesson not found":
			writeError(w, http.StatusNotFound, err.Error())
		case "unknown student":
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to save lesson records")
		}
		return
	}

	s.logInfo("server", "lesson.update", map[string]string{
		"groupId": groupID,
		"date":    lessonDate,
	})
	writeJSON(w, http.StatusOK, lesson)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	store, err := s.repo.ListGroups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export journal state")
		return
	}

	s.logInfo("server", "journal.export", map[string]string{
		"groups": fmt.Sprintf("%d", len(store.Groups)),
	})
	filename := fmt.Sprintf("journal-export-%s.json", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if err := json.NewEncoder(w).Encode(store); err != nil {
		http.Error(w, "failed to encode export", http.StatusInternalServerError)
	}
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	store, err := decodeImportedStore(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.repo.ImportStore(store); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to import journal state")
		return
	}

	s.logInfo("server", "journal.import", map[string]string{
		"groups": fmt.Sprintf("%d", len(store.Groups)),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "imported"})
}

func decodeImportedStore(r *http.Request) (Store, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			return Store{}, errors.New("journal file is required")
		}
		defer file.Close()
		return decodeStore(file)
	}

	return decodeStore(r.Body)
}

func decodeStore(reader io.Reader) (Store, error) {
	var store Store
	if err := json.NewDecoder(reader).Decode(&store); err != nil {
		return Store{}, errors.New("invalid journal file")
	}

	for gi := range store.Groups {
		group := &store.Groups[gi]
		group.Name = strings.TrimSpace(group.Name)
		group.Subject = strings.TrimSpace(group.Subject)
		if group.Name == "" {
			return Store{}, errors.New("group name is required")
		}
		if group.Subject == "" {
			return Store{}, errors.New("group subject is required")
		}
		if group.Students == nil {
			group.Students = []Student{}
		}
		if group.Lessons == nil {
			group.Lessons = []Lesson{}
		}

		cleanStudents := make([]Student, 0, len(group.Students))
		seenStudents := map[string]struct{}{}
		for _, student := range group.Students {
			name := strings.TrimSpace(student.Name)
			if name == "" {
				continue
			}
			id := strings.TrimSpace(student.ID)
			if id == "" {
				id = newID()
			}
			key := strings.ToLower(name)
			if _, exists := seenStudents[key]; exists {
				continue
			}
			seenStudents[key] = struct{}{}
			cleanStudents = append(cleanStudents, Student{
				ID:   id,
				Name: name,
			})
		}
		group.Students = cleanStudents
		if len(group.Students) == 0 {
			return Store{}, errors.New("at least one student is required")
		}

		allowedStudents := make(map[string]struct{}, len(group.Students))
		for _, student := range group.Students {
			allowedStudents[student.ID] = struct{}{}
		}

		for li := range group.Lessons {
			lesson := &group.Lessons[li]
			if _, err := time.Parse("2006-01-02", strings.TrimSpace(lesson.Date)); err != nil {
				return Store{}, errors.New("lesson date must be in YYYY-MM-DD format")
			}
			lesson.Theme = strings.TrimSpace(lesson.Theme)
			lesson.Term = strings.TrimSpace(lesson.Term)
			if lesson.Records == nil {
				lesson.Records = map[string]AttendanceRecord{}
			}

			records := make(map[string]AttendanceRecord, len(group.Students))
			for _, student := range group.Students {
				record := lesson.Records[student.ID]
				if record.Score != nil && (*record.Score < 1 || *record.Score > 5) {
					return Store{}, errors.New("score must be between 1 and 5")
				}
				records[student.ID] = record
			}
			for studentID := range lesson.Records {
				if _, ok := allowedStudents[studentID]; !ok {
					return Store{}, errors.New("unknown student")
				}
			}
			lesson.Records = records
		}
	}

	return store, nil
}

func normalizeStudents(names []string) []Student {
	students := make([]Student, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		students = append(students, Student{
			ID:   newID(),
			Name: trimmed,
		})
	}
	return students
}

func newID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

type LogEntry struct {
	ID      string            `json:"id"`
	Time    string            `json:"time"`
	Level   string            `json:"level"`
	Source  string            `json:"source"`
	Message string            `json:"message"`
	Meta    map[string]string `json:"meta,omitempty"`
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/logs") {
			next.ServeHTTP(w, r)
			return
		}

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)

		s.logInfo("request", "http.request", map[string]string{
			"method": r.Method,
			"path":   r.URL.Path,
			"status": fmt.Sprintf("%d", recorder.status),
			"ms":     fmt.Sprintf("%d", duration.Milliseconds()),
		})
	})
}

func (s *Server) logInfo(source, message string, meta map[string]string) {
	s.appendLog(LogEntry{
		ID:      newID(),
		Time:    time.Now().UTC().Format(time.RFC3339),
		Level:   "info",
		Source:  source,
		Message: message,
		Meta:    cloneLogMeta(meta),
	})
}

func (s *Server) appendLog(entry LogEntry) {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.logs == nil {
		s.logs = make([]LogEntry, 0, 200)
	}
	s.logs = append(s.logs, entry)
	if len(s.logs) > 200 {
		s.logs = s.logs[len(s.logs)-200:]
	}
}

func (s *Server) listLogs(limit int) []LogEntry {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if limit <= 0 || limit > len(s.logs) {
		limit = len(s.logs)
	}
	start := len(s.logs) - limit
	result := make([]LogEntry, limit)
	copy(result, s.logs[start:])
	return result
}

func cloneLogMeta(meta map[string]string) map[string]string {
	if meta == nil {
		return nil
	}
	copied := make(map[string]string, len(meta))
	for key, value := range meta {
		copied[key] = value
	}
	return copied
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := 200
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"logs": s.listLogs(limit),
		})
	case http.MethodPost:
		var req struct {
			Action string            `json:"action"`
			Detail string            `json:"detail"`
			Meta   map[string]string `json:"meta"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		action := strings.TrimSpace(req.Action)
		detail := strings.TrimSpace(req.Detail)
		if action == "" && detail == "" {
			writeError(w, http.StatusBadRequest, "action or detail is required")
			return
		}

		entry := LogEntry{
			ID:      newID(),
			Time:    time.Now().UTC().Format(time.RFC3339),
			Level:   "info",
			Source:  "client",
			Message: action,
			Meta:    cloneLogMeta(req.Meta),
		}
		if detail != "" {
			if entry.Meta == nil {
				entry.Meta = map[string]string{}
			}
			entry.Meta["detail"] = detail
		}
		if entry.Message == "" {
			entry.Message = "client.action"
		}
		s.appendLog(entry)
		writeJSON(w, http.StatusCreated, entry)
	default:
		writeMethodNotAllowed(w)
	}
}
