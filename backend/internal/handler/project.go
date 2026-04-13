package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ranit-biswas/taskflow/internal/model"
	"github.com/ranit-biswas/taskflow/internal/repository"
	"go.uber.org/zap"
)

type ProjectHandler struct {
	projects repository.ProjectRepository
	tasks    repository.TaskRepository
	logger   *zap.Logger
}

func NewProjectHandler(projects repository.ProjectRepository, tasks repository.TaskRepository, logger *zap.Logger) *ProjectHandler {
	return &ProjectHandler{projects: projects, tasks: tasks, logger: logger}
}

func (h *ProjectHandler) List(c echo.Context) error {
	userID := c.Get("user_id").(string)
	page, limit := parsePagination(c)

	projects, total, err := h.projects.ListByUser(c.Request().Context(), userID, page, limit)
	if err != nil {
		h.logger.Error("listing projects", zap.Error(err), zap.String("user_id", userID))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if projects == nil {
		projects = []model.Project{}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":  projects,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

type createProjectRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

func (h *ProjectHandler) Create(c echo.Context) error {
	var req createProjectRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  "validation failed",
			"fields": map[string]string{"name": "is required"},
		})
	}

	userID := c.Get("user_id").(string)
	project := &model.Project{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     userID,
		CreatedAt:   time.Now().UTC(),
	}

	if err := h.projects.Create(c.Request().Context(), project); err != nil {
		h.logger.Error("creating project", zap.Error(err), zap.String("user_id", userID))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	h.logger.Info("project created", zap.String("project_id", project.ID), zap.String("user_id", userID))
	return c.JSON(http.StatusCreated, project)
}

func (h *ProjectHandler) Get(c echo.Context) error {
	project, err := h.projects.GetByID(c.Request().Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		h.logger.Error("getting project", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	tasks, err := h.tasks.ListByProject(c.Request().Context(), project.ID)
	if err != nil {
		h.logger.Error("listing project tasks", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tasks == nil {
		tasks = []model.Task{}
	}

	return c.JSON(http.StatusOK, model.ProjectWithTasks{
		Project: *project,
		Tasks:   tasks,
	})
}

type updateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (h *ProjectHandler) Update(c echo.Context) error {
	userID := c.Get("user_id").(string)
	project, err := h.projects.GetByID(c.Request().Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		h.logger.Error("getting project for update", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if project.OwnerID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only the project owner can update this project"})
	}

	var req updateProjectRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.Name != nil {
		if *req.Name == "" {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "validation failed",
				"fields": map[string]string{"name": "cannot be empty"},
			})
		}
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = req.Description
	}

	if err := h.projects.Update(c.Request().Context(), project); err != nil {
		h.logger.Error("updating project", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	h.logger.Info("project updated", zap.String("project_id", project.ID), zap.String("user_id", userID))
	return c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) Delete(c echo.Context) error {
	userID := c.Get("user_id").(string)
	project, err := h.projects.GetByID(c.Request().Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		h.logger.Error("getting project for delete", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if project.OwnerID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only the project owner can delete this project"})
	}

	if err := h.projects.Delete(c.Request().Context(), project.ID); err != nil {
		h.logger.Error("deleting project", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	h.logger.Info("project deleted", zap.String("project_id", project.ID), zap.String("user_id", userID))
	return c.NoContent(http.StatusNoContent)
}

func (h *ProjectHandler) Stats(c echo.Context) error {
	projectID := c.Param("id")
	_, err := h.projects.GetByID(c.Request().Context(), projectID)
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		h.logger.Error("getting project for stats", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	stats, err := h.tasks.Stats(c.Request().Context(), projectID)
	if err != nil {
		h.logger.Error("getting project stats", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, stats)
}

func parsePagination(c echo.Context) (int, int) {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
