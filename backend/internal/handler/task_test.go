package handler_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/ranit-biswas/taskflow/internal/handler"
	"github.com/ranit-biswas/taskflow/internal/model"
	"github.com/ranit-biswas/taskflow/internal/repository"
	"github.com/ranit-biswas/taskflow/internal/repository/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTaskHandler(t *mocks.TaskRepo, p *mocks.ProjectRepo, u *mocks.UserRepo) *handler.TaskHandler {
	return handler.NewTaskHandler(t, p, u, zap.NewNop())
}

func sampleTask() *model.Task {
	return &model.Task{
		ID: "task-1", Title: "Do something", Status: "todo", Priority: "high",
		ProjectID: "proj-1", CreatorID: testUserID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

// --- Create ---

func TestTaskCreate_Success(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	projRepo := new(mocks.ProjectRepo)
	userRepo := new(mocks.UserRepo)
	h := newTaskHandler(taskRepo, projRepo, userRepo)

	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)
	taskRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	c, rec := authedContext("POST", "/projects/proj-1/tasks", map[string]string{
		"title": "New Task",
	})
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, "New Task", body["title"])
	assert.Equal(t, "todo", body["status"], "status defaults to todo")
	assert.Equal(t, "medium", body["priority"], "priority defaults to medium")
	assert.Equal(t, testUserID, body["creator_id"])
}

func TestTaskCreate_ProjectNotFound(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(new(mocks.TaskRepo), projRepo, new(mocks.UserRepo))

	projRepo.On("GetByID", mock.Anything, "nope").Return(nil, repository.ErrNotFound)

	c, rec := authedContext("POST", "/projects/nope/tasks", map[string]string{"title": "X"})
	c.SetParamNames("id")
	c.SetParamValues("nope")

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTaskCreate_MissingTitle(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(new(mocks.TaskRepo), projRepo, new(mocks.UserRepo))

	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)

	c, rec := authedContext("POST", "/projects/proj-1/tasks", map[string]string{"title": ""})
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	body := parseBody(rec)
	fields := body["fields"].(map[string]any)
	assert.Equal(t, "is required", fields["title"])
}

func TestTaskCreate_InvalidStatus(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(new(mocks.TaskRepo), projRepo, new(mocks.UserRepo))

	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)

	c, rec := authedContext("POST", "/projects/proj-1/tasks", map[string]string{
		"title": "X", "status": "invalid",
	})
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	body := parseBody(rec)
	fields := body["fields"].(map[string]any)
	assert.Contains(t, fields["status"], "must be one of")
}

func TestTaskCreate_InvalidPriority(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(new(mocks.TaskRepo), projRepo, new(mocks.UserRepo))

	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)

	c, rec := authedContext("POST", "/projects/proj-1/tasks", map[string]string{
		"title": "X", "priority": "critical",
	})
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	body := parseBody(rec)
	fields := body["fields"].(map[string]any)
	assert.Contains(t, fields["priority"], "must be one of")
}

func TestTaskCreate_AssigneeNotFound(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	projRepo := new(mocks.ProjectRepo)
	userRepo := new(mocks.UserRepo)
	h := newTaskHandler(taskRepo, projRepo, userRepo)

	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)
	userRepo.On("Exists", mock.Anything, "ghost-user").Return(false, nil)

	assignee := "ghost-user"
	c, rec := authedContext("POST", "/projects/proj-1/tasks", map[string]any{
		"title": "X", "assignee_id": assignee,
	})
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	body := parseBody(rec)
	fields := body["fields"].(map[string]any)
	assert.Equal(t, "user not found", fields["assignee_id"])
}

// --- List ---

func TestTaskList_Success(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(taskRepo, projRepo, new(mocks.UserRepo))

	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)
	taskRepo.On("List", mock.Anything, mock.MatchedBy(func(f repository.TaskFilter) bool {
		return f.ProjectID == "proj-1" && f.Status == "" && f.Page == 1
	})).Return([]model.Task{*sampleTask()}, 1, nil)

	c, rec := authedContext("GET", "/projects/proj-1/tasks", nil)
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, float64(1), body["total"])
	data := body["data"].([]any)
	assert.Len(t, data, 1)
}

func TestTaskList_WithStatusFilter(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(taskRepo, projRepo, new(mocks.UserRepo))

	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)
	taskRepo.On("List", mock.Anything, mock.MatchedBy(func(f repository.TaskFilter) bool {
		return f.Status == "done"
	})).Return([]model.Task{}, 0, nil)

	c, rec := authedContext("GET", "/projects/proj-1/tasks?status=done", nil)
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, float64(0), body["total"])
}

func TestTaskList_InvalidStatusFilter(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(new(mocks.TaskRepo), projRepo, new(mocks.UserRepo))

	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)

	c, rec := authedContext("GET", "/projects/proj-1/tasks?status=bogus", nil)
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTaskList_ProjectNotFound(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(new(mocks.TaskRepo), projRepo, new(mocks.UserRepo))

	projRepo.On("GetByID", mock.Anything, "nope").Return(nil, repository.ErrNotFound)

	c, rec := authedContext("GET", "/projects/nope/tasks", nil)
	c.SetParamNames("id")
	c.SetParamValues("nope")

	err := h.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Update ---

func TestTaskUpdate_Success(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	h := newTaskHandler(taskRepo, new(mocks.ProjectRepo), new(mocks.UserRepo))

	task := sampleTask()
	taskRepo.On("GetByID", mock.Anything, "task-1").Return(task, nil)
	taskRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	c, rec := authedContext("PATCH", "/tasks/task-1", map[string]string{"status": "done"})
	c.SetParamNames("id")
	c.SetParamValues("task-1")

	err := h.Update(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, "done", body["status"])
}

func TestTaskUpdate_NotFound(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	h := newTaskHandler(taskRepo, new(mocks.ProjectRepo), new(mocks.UserRepo))

	taskRepo.On("GetByID", mock.Anything, "nope").Return(nil, repository.ErrNotFound)

	c, rec := authedContext("PATCH", "/tasks/nope", map[string]string{"status": "done"})
	c.SetParamNames("id")
	c.SetParamValues("nope")

	err := h.Update(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTaskUpdate_InvalidStatus(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	h := newTaskHandler(taskRepo, new(mocks.ProjectRepo), new(mocks.UserRepo))

	taskRepo.On("GetByID", mock.Anything, "task-1").Return(sampleTask(), nil)

	c, rec := authedContext("PATCH", "/tasks/task-1", map[string]string{"status": "invalid"})
	c.SetParamNames("id")
	c.SetParamValues("task-1")

	err := h.Update(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTaskUpdate_EmptyTitle(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	h := newTaskHandler(taskRepo, new(mocks.ProjectRepo), new(mocks.UserRepo))

	taskRepo.On("GetByID", mock.Anything, "task-1").Return(sampleTask(), nil)

	title := ""
	c, rec := authedContext("PATCH", "/tasks/task-1", map[string]*string{"title": &title})
	c.SetParamNames("id")
	c.SetParamValues("task-1")

	err := h.Update(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	body := parseBody(rec)
	fields := body["fields"].(map[string]any)
	assert.Equal(t, "cannot be empty", fields["title"])
}

// --- Delete ---

func TestTaskDelete_ByProjectOwner(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(taskRepo, projRepo, new(mocks.UserRepo))

	task := sampleTask()
	task.CreatorID = "someone-else"
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject(testUserID), nil)
	taskRepo.On("GetByID", mock.Anything, "task-1").Return(task, nil)
	taskRepo.On("Delete", mock.Anything, "task-1").Return(nil)

	c, rec := authedContext("DELETE", "/tasks/task-1", nil)
	c.SetParamNames("id")
	c.SetParamValues("task-1")

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTaskDelete_ByTaskCreator(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(taskRepo, projRepo, new(mocks.UserRepo))

	task := sampleTask()
	task.CreatorID = testUserID
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject("other-owner"), nil)
	taskRepo.On("GetByID", mock.Anything, "task-1").Return(task, nil)
	taskRepo.On("Delete", mock.Anything, "task-1").Return(nil)

	c, rec := authedContext("DELETE", "/tasks/task-1", nil)
	c.SetParamNames("id")
	c.SetParamValues("task-1")

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTaskDelete_Forbidden(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	projRepo := new(mocks.ProjectRepo)
	h := newTaskHandler(taskRepo, projRepo, new(mocks.UserRepo))

	task := sampleTask()
	task.CreatorID = "someone-else"
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(sampleProject("other-owner"), nil)
	taskRepo.On("GetByID", mock.Anything, "task-1").Return(task, nil)

	c, rec := authedContext("DELETE", "/tasks/task-1", nil)
	c.SetParamNames("id")
	c.SetParamValues("task-1")

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	body := parseBody(rec)
	assert.Contains(t, body["error"], "project owner or task creator")
}

func TestTaskDelete_NotFound(t *testing.T) {
	taskRepo := new(mocks.TaskRepo)
	h := newTaskHandler(taskRepo, new(mocks.ProjectRepo), new(mocks.UserRepo))

	taskRepo.On("GetByID", mock.Anything, "nope").Return(nil, repository.ErrNotFound)

	c, rec := authedContext("DELETE", "/tasks/nope", nil)
	c.SetParamNames("id")
	c.SetParamValues("nope")

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
