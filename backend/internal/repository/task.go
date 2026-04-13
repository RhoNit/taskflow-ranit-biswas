package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ranit-biswas/taskflow/internal/model"
)

type TaskRepo struct {
	db *sql.DB
}

func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

const taskColumns = `id, title, description, status, priority, project_id, creator_id, assignee_id, due_date, created_at, updated_at`

func scanTask(row interface{ Scan(...any) error }) (*model.Task, error) {
	t := &model.Task{}
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.ProjectID, &t.CreatorID, &t.AssigneeID, &t.DueDate, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (r *TaskRepo) Create(ctx context.Context, t *model.Task) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO tasks (id, title, description, status, priority, project_id, creator_id, assignee_id, due_date, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING created_at, updated_at`,
		t.ID, t.Title, t.Description, t.Status, t.Priority,
		t.ProjectID, t.CreatorID, t.AssigneeID, t.DueDate, t.CreatedAt, t.UpdatedAt,
	).Scan(&t.CreatedAt, &t.UpdatedAt)
}

func (r *TaskRepo) GetByID(ctx context.Context, id string) (*model.Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE id = $1`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

type TaskFilter struct {
	ProjectID  string
	Status     string
	AssigneeID string
	Page       int
	Limit      int
}

func (r *TaskRepo) List(ctx context.Context, f TaskFilter) ([]model.Task, int, error) {
	where := []string{"project_id = $1"}
	args := []any{f.ProjectID}
	idx := 2

	if f.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, f.Status)
		idx++
	}
	if f.AssigneeID != "" {
		where = append(where, fmt.Sprintf("assignee_id = $%d", idx))
		args = append(args, f.AssigneeID)
		idx++
	}

	clause := strings.Join(where, " AND ")

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM tasks WHERE %s`, clause), countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (f.Page - 1) * f.Limit
	args = append(args, f.Limit, offset)
	query := fmt.Sprintf(
		`SELECT %s FROM tasks WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		taskColumns, clause, idx, idx+1,
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, total, rows.Err()
}

func (r *TaskRepo) Update(ctx context.Context, t *model.Task) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET title=$1, description=$2, status=$3, priority=$4,
		 assignee_id=$5, due_date=$6, updated_at=NOW()
		 WHERE id=$7`,
		t.Title, t.Description, t.Status, t.Priority,
		t.AssigneeID, t.DueDate, t.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepo) ListByProject(ctx context.Context, projectID string) ([]model.Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE project_id = $1 ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

type TaskStats struct {
	ByStatus   map[string]int `json:"by_status"`
	ByAssignee map[string]int `json:"by_assignee"`
}

func (r *TaskRepo) Stats(ctx context.Context, projectID string) (*TaskStats, error) {
	stats := &TaskStats{
		ByStatus:   make(map[string]int),
		ByAssignee: make(map[string]int),
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY status`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats.ByStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows2, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(assignee_id::TEXT, 'unassigned'), COUNT(*)
		 FROM tasks WHERE project_id = $1 GROUP BY assignee_id`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var assignee string
		var count int
		if err := rows2.Scan(&assignee, &count); err != nil {
			return nil, err
		}
		stats.ByAssignee[assignee] = count
	}
	return stats, rows2.Err()
}
