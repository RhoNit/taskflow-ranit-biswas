package handler_test

import (
	"errors"
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

func newProjectHandler(p *mocks.ProjectRepo, t *mocks.TaskRepo) *handler.ProjectHandler {
	return handler.NewProjectHandler(p, t, zap.NewNop())
}

func sampleProject(ownerID string) *model.Project {
	desc := "A test project"
	return &model.Project{
		ID: "proj-1", Name: "Test Project", Description: &desc,
		OwnerID: ownerID, CreatedAt: time.Now(),
	}
}

// --- Create ---

func TestProjectCreate_Success(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	projRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	c, rec := authedContext("POST", "/projects", map[string]string{"name": "New Project"})

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, "New Project", body["name"])
	assert.Equal(t, testUserID, body["owner_id"])

	projRepo.AssertExpectations(t)
}

func TestProjectCreate_EmptyName(t *testing.T) {
	h := newProjectHandler(new(mocks.ProjectRepo), new(mocks.TaskRepo))

	c, rec := authedContext("POST", "/projects", map[string]string{"name": ""})

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	body := parseBody(rec)
	fields := body["fields"].(map[string]any)
	assert.Equal(t, "is required", fields["name"])
}

func TestProjectCreate_DBError(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	projRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db down"))

	c, rec := authedContext("POST", "/projects", map[string]string{"name": "X"})

	err := h.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- List ---

func TestProjectList_Success(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	projects := []model.Project{*sampleProject(testUserID)}
	projRepo.On("ListByUser", mock.Anything, testUserID, 1, 20).Return(projects, 1, nil)

	c, rec := authedContext("GET", "/projects", nil)

	err := h.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, float64(1), body["total"])
	data := body["data"].([]any)
	assert.Len(t, data, 1)
}

func TestProjectList_Empty(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	projRepo.On("ListByUser", mock.Anything, testUserID, 1, 20).Return(nil, 0, nil)

	c, rec := authedContext("GET", "/projects", nil)

	err := h.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	data := body["data"].([]any)
	assert.Len(t, data, 0)
}

// --- Get ---

func TestProjectGet_Success(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	taskRepo := new(mocks.TaskRepo)
	h := newProjectHandler(projRepo, taskRepo)

	proj := sampleProject(testUserID)
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(proj, nil)
	taskRepo.On("ListByProject", mock.Anything, "proj-1").Return([]model.Task{
		{ID: "task-1", Title: "Task A", Status: "todo", ProjectID: "proj-1"},
	}, nil)

	c, rec := authedContext("GET", "/projects/proj-1", nil)
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Get(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, "Test Project", body["name"])
	tasks := body["tasks"].([]any)
	assert.Len(t, tasks, 1)
}

func TestProjectGet_NotFound(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	projRepo.On("GetByID", mock.Anything, "nope").Return(nil, repository.ErrNotFound)

	c, rec := authedContext("GET", "/projects/nope", nil)
	c.SetParamNames("id")
	c.SetParamValues("nope")

	err := h.Get(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Update ---

func TestProjectUpdate_Success(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	proj := sampleProject(testUserID)
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(proj, nil)
	projRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	c, rec := authedContext("PATCH", "/projects/proj-1", map[string]string{"name": "Renamed"})
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Update(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, "Renamed", body["name"])
}

func TestProjectUpdate_NotOwner(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	proj := sampleProject("other-user")
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(proj, nil)

	c, rec := authedContext("PATCH", "/projects/proj-1", map[string]string{"name": "Renamed"})
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Update(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestProjectUpdate_NotFound(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	projRepo.On("GetByID", mock.Anything, "nope").Return(nil, repository.ErrNotFound)

	c, rec := authedContext("PATCH", "/projects/nope", map[string]string{"name": "X"})
	c.SetParamNames("id")
	c.SetParamValues("nope")

	err := h.Update(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestProjectUpdate_EmptyName(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	proj := sampleProject(testUserID)
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(proj, nil)

	name := ""
	c, rec := authedContext("PATCH", "/projects/proj-1", map[string]*string{"name": &name})
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Update(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Delete ---

func TestProjectDelete_Success(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	proj := sampleProject(testUserID)
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(proj, nil)
	projRepo.On("Delete", mock.Anything, "proj-1").Return(nil)

	c, rec := authedContext("DELETE", "/projects/proj-1", nil)
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestProjectDelete_NotOwner(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	proj := sampleProject("other-user")
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(proj, nil)

	c, rec := authedContext("DELETE", "/projects/proj-1", nil)
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestProjectDelete_NotFound(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	projRepo.On("GetByID", mock.Anything, "nope").Return(nil, repository.ErrNotFound)

	c, rec := authedContext("DELETE", "/projects/nope", nil)
	c.SetParamNames("id")
	c.SetParamValues("nope")

	err := h.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Stats ---

func TestProjectStats_Success(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	taskRepo := new(mocks.TaskRepo)
	h := newProjectHandler(projRepo, taskRepo)

	proj := sampleProject(testUserID)
	projRepo.On("GetByID", mock.Anything, "proj-1").Return(proj, nil)
	taskRepo.On("Stats", mock.Anything, "proj-1").Return(&repository.TaskStats{
		ByStatus:   map[string]int{"todo": 2, "done": 1},
		ByAssignee: map[string]int{testUserID: 2, "unassigned": 1},
	}, nil)

	c, rec := authedContext("GET", "/projects/proj-1/stats", nil)
	c.SetParamNames("id")
	c.SetParamValues("proj-1")

	err := h.Stats(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	byStatus := body["by_status"].(map[string]any)
	assert.Equal(t, float64(2), byStatus["todo"])
	assert.Equal(t, float64(1), byStatus["done"])
}

func TestProjectStats_NotFound(t *testing.T) {
	projRepo := new(mocks.ProjectRepo)
	h := newProjectHandler(projRepo, new(mocks.TaskRepo))

	projRepo.On("GetByID", mock.Anything, "nope").Return(nil, repository.ErrNotFound)

	c, rec := authedContext("GET", "/projects/nope/stats", nil)
	c.SetParamNames("id")
	c.SetParamValues("nope")

	err := h.Stats(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
