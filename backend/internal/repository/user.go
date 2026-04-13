package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ranit-biswas/taskflow/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicateEmail = errors.New("email already exists")

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (id, name, email, password, created_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		u.ID, u.Name, u.Email, u.Password, u.CreatedAt,
	).Scan(&u.CreatedAt)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateEmail
		}
		return err
	}
	return nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id,
	).Scan(&exists)
	return exists, err
}
