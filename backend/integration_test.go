package taskflow_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/ranit-biswas/taskflow/internal/handler"
	"github.com/ranit-biswas/taskflow/internal/middleware"
	"github.com/ranit-biswas/taskflow/internal/repository"
	"github.com/ranit-biswas/taskflow/migrations"
	"go.uber.org/zap"
)

var (
	testDB     *sql.DB
	testEcho   *echo.Echo
	jwtSecret  = "test-secret"
	testLogger = zap.NewNop()
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Println("SKIP: TEST_DATABASE_URL not set")
		os.Exit(0)
	}

	var err error
	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db: %v\n", err)
		os.Exit(1)
	}
	defer testDB.Close()

	goose.SetBaseFS(migrations.FS)
	_ = goose.SetDialect("postgres")
	if err := goose.Up(testDB, "."); err != nil {
		fmt.Fprintf(os.Stderr, "failed to migrate: %v\n", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepo(testDB)
	projectRepo := repository.NewProjectRepo(testDB)
	taskRepo := repository.NewTaskRepo(testDB)

	authH := handler.NewAuthHandler(userRepo, jwtSecret, testLogger)
	projectH := handler.NewProjectHandler(projectRepo, taskRepo, testLogger)
	taskH := handler.NewTaskHandler(taskRepo, projectRepo, userRepo, testLogger)

	testEcho = echo.New()
	testEcho.POST("/auth/register", authH.Register)
	testEcho.POST("/auth/login", authH.Login)

	api := testEcho.Group("")
	api.Use(middleware.JWTAuth(jwtSecret))
	api.GET("/projects", projectH.List)
	api.POST("/projects", projectH.Create)
	api.GET("/projects/:id", projectH.Get)
	api.PATCH("/projects/:id", projectH.Update)
	api.DELETE("/projects/:id", projectH.Delete)
	api.GET("/projects/:id/tasks", taskH.List)
	api.POST("/projects/:id/tasks", taskH.Create)
	api.PATCH("/tasks/:id", taskH.Update)
	api.DELETE("/tasks/:id", taskH.Delete)

	code := m.Run()

	_ = goose.Reset(testDB, ".")
	os.Exit(code)
}

func doRequest(method, path string, body any, token string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	testEcho.ServeHTTP(rec, req)
	return rec
}

func parseJSON(rec *httptest.ResponseRecorder) map[string]any {
	var result map[string]any
	json.Unmarshal(rec.Body.Bytes(), &result)
	return result
}

// Test 1: Registration and login flow
func TestAuthFlow(t *testing.T) {
	rec := doRequest("POST", "/auth/register", map[string]string{
		"name": "Integration Test", "email": "integ@test.com", "password": "testpass123",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	body := parseJSON(rec)
	if body["email"] != "integ@test.com" {
		t.Fatalf("register: expected email integ@test.com, got %v", body["email"])
	}

	rec = doRequest("POST", "/auth/register", map[string]string{
		"name": "Dup", "email": "integ@test.com", "password": "testpass123",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate register: expected 400, got %d", rec.Code)
	}

	rec = doRequest("POST", "/auth/login", map[string]string{
		"email": "integ@test.com", "password": "testpass123",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	loginBody := parseJSON(rec)
	if loginBody["token"] == nil || loginBody["token"] == "" {
		t.Fatal("login: expected token in response")
	}

	rec = doRequest("POST", "/auth/login", map[string]string{
		"email": "integ@test.com", "password": "wrongpassword",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", rec.Code)
	}
}

func login(t *testing.T, email, password string) string {
	t.Helper()
	rec := doRequest("POST", "/auth/login", map[string]string{
		"email": email, "password": password,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	return parseJSON(rec)["token"].(string)
}

// Test 2: Project CRUD with authorization
func TestProjectCRUD(t *testing.T) {
	doRequest("POST", "/auth/register", map[string]string{
		"name": "ProjUser", "email": "proj@test.com", "password": "testpass123",
	}, "")
	token := login(t, "proj@test.com", "testpass123")

	rec := doRequest("POST", "/projects", map[string]string{"name": "Test Project"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	proj := parseJSON(rec)
	projectID := proj["id"].(string)

	rec = doRequest("GET", "/projects/"+projectID, nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("get project: expected 200, got %d", rec.Code)
	}

	rec = doRequest("PATCH", "/projects/"+projectID, map[string]string{"name": "Updated"}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("update project: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated := parseJSON(rec)
	if updated["name"] != "Updated" {
		t.Fatalf("update project: expected name Updated, got %v", updated["name"])
	}

	rec = doRequest("GET", "/projects", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list projects: expected 200, got %d", rec.Code)
	}

	doRequest("POST", "/auth/register", map[string]string{
		"name": "Other", "email": "other@test.com", "password": "testpass123",
	}, "")
	otherToken := login(t, "other@test.com", "testpass123")

	rec = doRequest("DELETE", "/projects/"+projectID, nil, otherToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("delete by non-owner: expected 403, got %d", rec.Code)
	}

	rec = doRequest("DELETE", "/projects/"+projectID, nil, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete project: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Test 3: Task CRUD with filters
func TestTaskCRUD(t *testing.T) {
	doRequest("POST", "/auth/register", map[string]string{
		"name": "TaskUser", "email": "task@test.com", "password": "testpass123",
	}, "")
	token := login(t, "task@test.com", "testpass123")

	rec := doRequest("POST", "/projects", map[string]string{"name": "Task Project"}, token)
	projectID := parseJSON(rec)["id"].(string)

	rec = doRequest("POST", "/projects/"+projectID+"/tasks", map[string]any{
		"title": "Task A", "status": "todo", "priority": "high",
	}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	taskA := parseJSON(rec)
	taskAID := taskA["id"].(string)

	doRequest("POST", "/projects/"+projectID+"/tasks", map[string]any{
		"title": "Task B", "status": "done", "priority": "low",
	}, token)

	rec = doRequest("GET", "/projects/"+projectID+"/tasks", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list tasks: expected 200, got %d", rec.Code)
	}
	list := parseJSON(rec)
	data := list["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("list tasks: expected 2 tasks, got %d", len(data))
	}

	rec = doRequest("GET", "/projects/"+projectID+"/tasks?status=todo", nil, token)
	filtered := parseJSON(rec)
	filteredData := filtered["data"].([]any)
	if len(filteredData) != 1 {
		t.Fatalf("filter tasks: expected 1 task, got %d", len(filteredData))
	}

	rec = doRequest("PATCH", "/tasks/"+taskAID, map[string]any{
		"status": "in_progress", "title": "Task A Updated",
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("update task: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updatedTask := parseJSON(rec)
	if updatedTask["status"] != "in_progress" {
		t.Fatalf("update task: expected in_progress, got %v", updatedTask["status"])
	}

	rec = doRequest("DELETE", "/tasks/"+taskAID, nil, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete task: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest("GET", "/projects/"+projectID+"/tasks", nil, token)
	remaining := parseJSON(rec)
	remainingData := remaining["data"].([]any)
	if len(remainingData) != 1 {
		t.Fatalf("after delete: expected 1 task remaining, got %d", len(remainingData))
	}
}

// Test 4: Validation errors
func TestValidation(t *testing.T) {
	rec := doRequest("POST", "/auth/register", map[string]string{
		"name": "", "email": "", "password": "",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("validation: expected 400, got %d", rec.Code)
	}
	body := parseJSON(rec)
	fields, ok := body["fields"].(map[string]any)
	if !ok || len(fields) == 0 {
		t.Fatal("validation: expected fields in error response")
	}

	rec = doRequest("GET", "/projects", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: expected 401, got %d", rec.Code)
	}

	rec = doRequest("GET", "/projects", nil, "invalid-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: expected 401, got %d", rec.Code)
	}
}
