package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func intPtr(v int) *int {
	return &v
}

func TestGroupLessonFlow(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	groupBody := bytes.NewBufferString(`{"name":"Group A","subject":"Math","students":["Alice","Bob"]}`)
	groupReq := httptest.NewRequest(http.MethodPost, "/api/groups", groupBody)
	groupRec := httptest.NewRecorder()
	server.handleGroups(groupRec, groupReq)
	if groupRec.Code != http.StatusCreated {
		t.Fatalf("create group status = %d, want %d", groupRec.Code, http.StatusCreated)
	}

	var groupResp Group
	if err := json.Unmarshal(groupRec.Body.Bytes(), &groupResp); err != nil {
		t.Fatalf("unmarshal group: %v", err)
	}
	if len(groupResp.Students) != 2 {
		t.Fatalf("students len = %d, want 2", len(groupResp.Students))
	}

	lessonReq := httptest.NewRequest(http.MethodPost, "/api/groups/"+groupResp.ID+"/lessons", bytes.NewBufferString(`{"date":"2026-03-01"}`))
	lessonRec := httptest.NewRecorder()
	server.handleGroupRoutes(lessonRec, lessonReq)
	if lessonRec.Code != http.StatusCreated {
		t.Fatalf("create lesson status = %d, want %d", lessonRec.Code, http.StatusCreated)
	}

	var lessonResp Lesson
	if err := json.Unmarshal(lessonRec.Body.Bytes(), &lessonResp); err != nil {
		t.Fatalf("unmarshal lesson: %v", err)
	}
	if lessonResp.Date != "2026-03-01" {
		t.Fatalf("lesson date = %q, want %q", lessonResp.Date, "2026-03-01")
	}

	updatePayload := map[string]map[string]AttendanceRecord{
		"records": {
			groupResp.Students[0].ID: {
				Present: true,
				Score:   intPtr(5),
			},
			groupResp.Students[1].ID: {
				Present: false,
				Score:   nil,
			},
		},
	}
	body, _ := json.Marshal(map[string]any{
		"theme":   "Control work",
		"term":    "Spring",
		"records": updatePayload["records"],
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/groups/"+groupResp.ID+"/lessons/"+lessonResp.Date+"/records", bytes.NewReader(body))
	updateRec := httptest.NewRecorder()
	server.handleGroupRoutes(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update lesson status = %d, want %d", updateRec.Code, http.StatusOK)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	listRec := httptest.NewRecorder()
	server.handleGroups(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list groups status = %d, want %d", listRec.Code, http.StatusOK)
	}

	var store Store
	if err := json.Unmarshal(listRec.Body.Bytes(), &store); err != nil {
		t.Fatalf("unmarshal store: %v", err)
	}
	if len(store.Groups) != 1 {
		t.Fatalf("groups len = %d, want 1", len(store.Groups))
	}
	gotScore := store.Groups[0].Lessons[0].Records[groupResp.Students[0].ID].Score
	if gotScore == nil || *gotScore != 5 {
		t.Fatalf("score = %v, want 5", gotScore)
	}
	if got := store.Groups[0].Lessons[0].Theme; got != "Control work" {
		t.Fatalf("theme = %q, want %q", got, "Control work")
	}
	if got := store.Groups[0].Lessons[0].Term; got != "Spring" {
		t.Fatalf("term = %q, want %q", got, "Spring")
	}
}

func TestCreateLessonRejectsInvalidDate(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	groupBody := bytes.NewBufferString(`{"name":"Group B","subject":"History","students":["Ann"]}`)
	groupReq := httptest.NewRequest(http.MethodPost, "/api/groups", groupBody)
	groupRec := httptest.NewRecorder()
	server.handleGroups(groupRec, groupReq)
	if groupRec.Code != http.StatusCreated {
		t.Fatalf("create group status = %d, want %d", groupRec.Code, http.StatusCreated)
	}

	var groupResp Group
	if err := json.Unmarshal(groupRec.Body.Bytes(), &groupResp); err != nil {
		t.Fatalf("unmarshal group: %v", err)
	}

	lessonReq := httptest.NewRequest(http.MethodPost, "/api/groups/"+groupResp.ID+"/lessons", bytes.NewBufferString(`{"date":"03-01-2026"}`))
	lessonRec := httptest.NewRecorder()
	server.handleGroupRoutes(lessonRec, lessonReq)
	if lessonRec.Code != http.StatusBadRequest {
		t.Fatalf("create lesson status = %d, want %d", lessonRec.Code, http.StatusBadRequest)
	}
}

func TestDeleteGroupFlow(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	groupReq := httptest.NewRequest(http.MethodPost, "/api/groups", bytes.NewBufferString(`{"name":"Group D","subject":"Biology","students":["Ira"]}`))
	groupRec := httptest.NewRecorder()
	server.handleGroups(groupRec, groupReq)
	if groupRec.Code != http.StatusCreated {
		t.Fatalf("create group status = %d, want %d", groupRec.Code, http.StatusCreated)
	}

	var groupResp Group
	if err := json.Unmarshal(groupRec.Body.Bytes(), &groupResp); err != nil {
		t.Fatalf("unmarshal group: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/groups/"+groupResp.ID, nil)
	deleteRec := httptest.NewRecorder()
	server.handleGroupRoutes(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete group status = %d, want %d", deleteRec.Code, http.StatusOK)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/groups/"+groupResp.ID, nil)
	getRec := httptest.NewRecorder()
	server.handleGroupRoutes(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("get deleted group status = %d, want %d", getRec.Code, http.StatusNotFound)
	}
}

func TestImportExportFlow(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	importBody := bytes.NewBufferString(`{
		"groups": [{
			"name": "Group C",
			"subject": "Physics",
			"students": [{"id":"s1","name":"Eva"}],
			"lessons": [{
				"date":"2026-03-02",
				"theme":"Lab work",
				"term":"Quarter 3",
				"records": {"s1":{"present":true,"score":4}}
			}]
		}]
	}`)
	importReq := httptest.NewRequest(http.MethodPost, "/api/journal/import", importBody)
	importReq.Header.Set("Content-Type", "application/json")
	importRec := httptest.NewRecorder()
	server.handleImport(importRec, importReq)
	if importRec.Code != http.StatusOK {
		t.Fatalf("import status = %d, want %d", importRec.Code, http.StatusOK)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/journal/export", nil)
	exportRec := httptest.NewRecorder()
	server.handleExport(exportRec, exportReq)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d", exportRec.Code, http.StatusOK)
	}

	var store Store
	if err := json.Unmarshal(exportRec.Body.Bytes(), &store); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	if len(store.Groups) != 1 {
		t.Fatalf("groups len = %d, want 1", len(store.Groups))
	}
	if got := store.Groups[0].Lessons[0].Theme; got != "Lab work" {
		t.Fatalf("theme = %q, want %q", got, "Lab work")
	}
	score := store.Groups[0].Lessons[0].Records[store.Groups[0].Students[0].ID].Score
	if score == nil || *score != 4 {
		t.Fatalf("score = %v, want 4", score)
	}
}

func TestRegisterUserFlow(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"email":"teacher@example.com","password":"strongpass"}`))
	rec := httptest.NewRecorder()
	server.handleRegister(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var user User
	if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	if user.Email != "teacher@example.com" {
		t.Fatalf("email = %q, want %q", user.Email, "teacher@example.com")
	}
	if user.ID == "" {
		t.Fatal("expected user id to be set")
	}
}

func TestRegisterRejectsInvalidEmail(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"email":"teacher","password":"strongpass"}`))
	rec := httptest.NewRecorder()
	server.handleRegister(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("register status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	first := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"email":"teacher@example.com","password":"strongpass"}`))
	firstRec := httptest.NewRecorder()
	server.handleRegister(firstRec, first)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("first register status = %d, want %d", firstRec.Code, http.StatusCreated)
	}

	second := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"email":"teacher@example.com","password":"anotherpass"}`))
	secondRec := httptest.NewRecorder()
	server.handleRegister(secondRec, second)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want %d", secondRec.Code, http.StatusConflict)
	}
}

func TestLoginUserFlow(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	registerReq := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"email":"teacher@example.com","password":"strongpass"}`))
	registerRec := httptest.NewRecorder()
	server.handleRegister(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerRec.Code, http.StatusCreated)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"email":"teacher@example.com","password":"strongpass"}`))
	loginRec := httptest.NewRecorder()
	server.handleLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusOK)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	server := NewServerWithRepository(NewMemoryRepository())

	registerReq := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"email":"teacher@example.com","password":"strongpass"}`))
	registerRec := httptest.NewRecorder()
	server.handleRegister(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerRec.Code, http.StatusCreated)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"email":"teacher@example.com","password":"wrongpass"}`))
	loginRec := httptest.NewRecorder()
	server.handleLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusUnauthorized)
	}
}
