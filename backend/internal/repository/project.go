package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ranit-biswas/taskflow/internal/model"
)

type ProjectRepo struct {
	db *sql.DB
}

func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

func (r *ProjectRepo) Create(ctx context.Context, p *model.Project) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO projects (id, name, description, owner_id, created_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING created_at`,
		p.ID, p.Name, p.Description, p.OwnerID, p.CreatedAt,
	).Scan(&p.CreatedAt)
}

func (r *ProjectRepo) GetByID(ctx context.Context, id string) (*model.Project, error) {
	p := &model.Project{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, description, owner_id, created_at FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ListByUser returns projects the user owns or has tasks assigned in.
func (r *ProjectRepo) ListByUser(ctx context.Context, userID string, page, limit int) ([]model.Project, int, error) {
	offset := (page - 1) * limit

	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT p.id) FROM projects p
		 LEFT JOIN tasks t ON t.project_id = p.id
		 WHERE p.owner_id = $1 OR t.assignee_id = $1`, userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT p.id, p.name, p.description, p.owner_id, p.created_at
		 FROM projects p
		 LEFT JOIN tasks t ON t.project_id = p.id
		 WHERE p.owner_id = $1 OR t.assignee_id = $1
		 ORDER BY p.created_at DESC
		 LIMIT $2 OFFSET $3`, userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		projects = append(projects, p)
	}
	return projects, total, rows.Err()
}

func (r *ProjectRepo) Update(ctx context.Context, p *model.Project) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE projects SET name = $1, description = $2 WHERE id = $3`,
		p.Name, p.Description, p.ID,
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

func (r *ProjectRepo) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE project_id = $1`, id); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}
