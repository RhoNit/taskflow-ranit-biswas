package repository

import (
	"context"

	"github.com/ranit-biswas/taskflow/internal/model"
)

type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Exists(ctx context.Context, id string) (bool, error)
}

type ProjectRepository interface {
	Create(ctx context.Context, p *model.Project) error
	GetByID(ctx context.Context, id string) (*model.Project, error)
	ListByUser(ctx context.Context, userID string, page, limit int) ([]model.Project, int, error)
	Update(ctx context.Context, p *model.Project) error
	Delete(ctx context.Context, id string) error
}

type TaskRepository interface {
	Create(ctx context.Context, t *model.Task) error
	GetByID(ctx context.Context, id string) (*model.Task, error)
	List(ctx context.Context, f TaskFilter) ([]model.Task, int, error)
	Update(ctx context.Context, t *model.Task) error
	Delete(ctx context.Context, id string) error
	ListByProject(ctx context.Context, projectID string) ([]model.Task, error)
	Stats(ctx context.Context, projectID string) (*TaskStats, error)
}
