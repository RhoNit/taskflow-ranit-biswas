package mocks

import (
	"context"

	"github.com/ranit-biswas/taskflow/internal/model"
	"github.com/ranit-biswas/taskflow/internal/repository"
	"github.com/stretchr/testify/mock"
)

// UserRepo mocks repository.UserRepository.
type UserRepo struct {
	mock.Mock
}

func (m *UserRepo) Create(ctx context.Context, u *model.User) error {
	return m.Called(ctx, u).Error(0)
}

func (m *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *UserRepo) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

// ProjectRepo mocks repository.ProjectRepository.
type ProjectRepo struct {
	mock.Mock
}

func (m *ProjectRepo) Create(ctx context.Context, p *model.Project) error {
	return m.Called(ctx, p).Error(0)
}

func (m *ProjectRepo) GetByID(ctx context.Context, id string) (*model.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Project), args.Error(1)
}

func (m *ProjectRepo) ListByUser(ctx context.Context, userID string, page, limit int) ([]model.Project, int, error) {
	args := m.Called(ctx, userID, page, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]model.Project), args.Int(1), args.Error(2)
}

func (m *ProjectRepo) Update(ctx context.Context, p *model.Project) error {
	return m.Called(ctx, p).Error(0)
}

func (m *ProjectRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

// TaskRepo mocks repository.TaskRepository.
type TaskRepo struct {
	mock.Mock
}

func (m *TaskRepo) Create(ctx context.Context, t *model.Task) error {
	return m.Called(ctx, t).Error(0)
}

func (m *TaskRepo) GetByID(ctx context.Context, id string) (*model.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Task), args.Error(1)
}

func (m *TaskRepo) List(ctx context.Context, f repository.TaskFilter) ([]model.Task, int, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]model.Task), args.Int(1), args.Error(2)
}

func (m *TaskRepo) Update(ctx context.Context, t *model.Task) error {
	return m.Called(ctx, t).Error(0)
}

func (m *TaskRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func (m *TaskRepo) ListByProject(ctx context.Context, projectID string) ([]model.Task, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Task), args.Error(1)
}

func (m *TaskRepo) Stats(ctx context.Context, projectID string) (*repository.TaskStats, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.TaskStats), args.Error(1)
}
