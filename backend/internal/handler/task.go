package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ranit-biswas/taskflow/internal/model"
	"github.com/ranit-biswas/taskflow/internal/repository"
	"go.uber.org/zap"
)

var validStatuses = map[string]bool{"todo": true, "in_progress": true, "done": true}
var validPriorities = map[string]bool{"low": true, "medium": true, "high": true}

type TaskHandler struct {
	tasks    repository.TaskRepository
	projects repository.ProjectRepository
	users    repository.UserRepository
	logger   *zap.Logger
}

func NewTaskHandler(tasks repository.TaskRepository, projects repository.ProjectRepository, users repository.UserRepository, logger *zap.Logger) *TaskHandler {
	return &TaskHandler{tasks: tasks, projects: projects, users: users, logger: logger}
}

func (h *TaskHandler) List(c echo.Context) error {
	projectID := c.Param("id")
	if _, err := h.projects.GetByID(c.Request().Context(), projectID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "project not found"})
		}
		h.logger.Error("getting project for task list", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	page, limit := parsePagination(c)
	status := c.QueryParam("status")
	if status != "" && !validStatuses[status] {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  "validation failed",
			"fields": map[string]string{"status": "must be one of: todo, in_progress, done"},
		})
	}

	assignee := c.QueryParam("assignee")

	tasks, total, err := h.tasks.List(c.Request().Context(), repository.TaskFilter{
		ProjectID:  projectID,
		Status:     status,
		AssigneeID: assignee,
		Page:       page,
		Limit:      limit,
	})
	if err != nil {
		h.logger.Error("listing tasks", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tasks == nil {
		tasks = []model.Task{}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":  tasks,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

type createTaskRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

func (h *TaskHandler) Create(c echo.Context) error {
	projectID := c.Param("id")
	if _, err := h.projects.GetByID(c.Request().Context(), projectID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "project not found"})
		}
		h.logger.Error("getting project for task creation", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	var req createTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	fieldErrors := make(map[string]string)
	if req.Title == "" {
		fieldErrors["title"] = "is required"
	}
	if req.Status == "" {
		req.Status = "todo"
	} else if !validStatuses[req.Status] {
		fieldErrors["status"] = "must be one of: todo, in_progress, done"
	}
	if req.Priority == "" {
		req.Priority = "medium"
	} else if !validPriorities[req.Priority] {
		fieldErrors["priority"] = "must be one of: low, medium, high"
	}
	if req.AssigneeID != nil && *req.AssigneeID != "" {
		exists, err := h.users.Exists(c.Request().Context(), *req.AssigneeID)
		if err != nil {
			h.logger.Error("checking assignee", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		if !exists {
			fieldErrors["assignee_id"] = "user not found"
		}
	}
	if len(fieldErrors) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  "validation failed",
			"fields": fieldErrors,
		})
	}

	userID := c.Get("user_id").(string)
	now := time.Now().UTC()
	task := &model.Task{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		ProjectID:   projectID,
		CreatorID:   userID,
		AssigneeID:  req.AssigneeID,
		DueDate:     req.DueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.tasks.Create(c.Request().Context(), task); err != nil {
		h.logger.Error("creating task", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	h.logger.Info("task created",
		zap.String("task_id", task.ID),
		zap.String("project_id", projectID),
		zap.String("user_id", userID),
	)
	return c.JSON(http.StatusCreated, task)
}

type updateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

func (h *TaskHandler) Update(c echo.Context) error {
	task, err := h.tasks.GetByID(c.Request().Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		h.logger.Error("getting task for update", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	var req updateTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	fieldErrors := make(map[string]string)
	if req.Title != nil {
		if *req.Title == "" {
			fieldErrors["title"] = "cannot be empty"
		} else {
			task.Title = *req.Title
		}
	}
	if req.Description != nil {
		task.Description = req.Description
	}
	if req.Status != nil {
		if !validStatuses[*req.Status] {
			fieldErrors["status"] = "must be one of: todo, in_progress, done"
		} else {
			task.Status = *req.Status
		}
	}
	if req.Priority != nil {
		if !validPriorities[*req.Priority] {
			fieldErrors["priority"] = "must be one of: low, medium, high"
		} else {
			task.Priority = *req.Priority
		}
	}
	if req.AssigneeID != nil {
		if *req.AssigneeID == "" {
			task.AssigneeID = nil
		} else {
			exists, err := h.users.Exists(c.Request().Context(), *req.AssigneeID)
			if err != nil {
				h.logger.Error("checking assignee", zap.Error(err))
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
			if !exists {
				fieldErrors["assignee_id"] = "user not found"
			} else {
				task.AssigneeID = req.AssigneeID
			}
		}
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
	}
	if len(fieldErrors) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  "validation failed",
			"fields": fieldErrors,
		})
	}

	if err := h.tasks.Update(c.Request().Context(), task); err != nil {
		h.logger.Error("updating task", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	h.logger.Info("task updated", zap.String("task_id", task.ID))
	return c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c echo.Context) error {
	userID := c.Get("user_id").(string)

	task, err := h.tasks.GetByID(c.Request().Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		h.logger.Error("getting task for delete", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	project, err := h.projects.GetByID(c.Request().Context(), task.ProjectID)
	if err != nil {
		h.logger.Error("getting project for task delete auth", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	isOwner := project.OwnerID == userID
	isCreator := task.CreatorID == userID
	if !isOwner && !isCreator {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "only the project owner or task creator can delete this task",
		})
	}

	if err := h.tasks.Delete(c.Request().Context(), task.ID); err != nil {
		h.logger.Error("deleting task", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	h.logger.Info("task deleted", zap.String("task_id", task.ID), zap.String("user_id", userID))
	return c.NoContent(http.StatusNoContent)
}
