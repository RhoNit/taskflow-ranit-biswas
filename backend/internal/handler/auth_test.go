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
	"golang.org/x/crypto/bcrypt"
)

func TestRegister_Success(t *testing.T) {
	userRepo := new(mocks.UserRepo)
	h := handler.NewAuthHandler(userRepo, testJWTSecret, zap.NewNop())

	userRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	c, rec := newContext("POST", "/auth/register", map[string]string{
		"name": "Jane", "email": "jane@test.com", "password": "password123",
	})

	err := h.Register(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, "Jane", body["name"])
	assert.Equal(t, "jane@test.com", body["email"])
	assert.NotEmpty(t, body["id"])
	assert.Nil(t, body["password"], "password must not appear in response")

	userRepo.AssertExpectations(t)
}

func TestRegister_ValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]string
		field  string
		reason string
	}{
		{"empty name", map[string]string{"name": "", "email": "a@b.com", "password": "123456"}, "name", "is required"},
		{"empty email", map[string]string{"name": "X", "email": "", "password": "123456"}, "email", "is required"},
		{"empty password", map[string]string{"name": "X", "email": "a@b.com", "password": ""}, "password", "is required"},
		{"short password", map[string]string{"name": "X", "email": "a@b.com", "password": "abc"}, "password", "must be at least 6 characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.UserRepo)
			h := handler.NewAuthHandler(userRepo, testJWTSecret, zap.NewNop())

			c, rec := newContext("POST", "/auth/register", tt.input)

			err := h.Register(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, rec.Code)

			body := parseBody(rec)
			fields := body["fields"].(map[string]any)
			assert.Equal(t, tt.reason, fields[tt.field])
		})
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	userRepo := new(mocks.UserRepo)
	h := handler.NewAuthHandler(userRepo, testJWTSecret, zap.NewNop())

	userRepo.On("Create", mock.Anything, mock.Anything).Return(repository.ErrDuplicateEmail)

	c, rec := newContext("POST", "/auth/register", map[string]string{
		"name": "Jane", "email": "dup@test.com", "password": "password123",
	})

	err := h.Register(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	body := parseBody(rec)
	fields := body["fields"].(map[string]any)
	assert.Equal(t, "already exists", fields["email"])
}

func TestRegister_DBError(t *testing.T) {
	userRepo := new(mocks.UserRepo)
	h := handler.NewAuthHandler(userRepo, testJWTSecret, zap.NewNop())

	userRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("connection refused"))

	c, rec := newContext("POST", "/auth/register", map[string]string{
		"name": "Jane", "email": "jane@test.com", "password": "password123",
	})

	err := h.Register(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestLogin_Success(t *testing.T) {
	userRepo := new(mocks.UserRepo)
	h := handler.NewAuthHandler(userRepo, testJWTSecret, zap.NewNop())

	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	userRepo.On("GetByEmail", mock.Anything, "jane@test.com").Return(&model.User{
		ID: "user-1", Name: "Jane", Email: "jane@test.com",
		Password: string(hashed), CreatedAt: time.Now(),
	}, nil)

	c, rec := newContext("POST", "/auth/login", map[string]string{
		"email": "jane@test.com", "password": "password123",
	})

	err := h.Login(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := parseBody(rec)
	assert.NotEmpty(t, body["token"])

	userRepo.AssertExpectations(t)
}

func TestLogin_ValidationErrors(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		field string
	}{
		{"empty email", map[string]string{"email": "", "password": "123456"}, "email"},
		{"empty password", map[string]string{"email": "a@b.com", "password": ""}, "password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.UserRepo)
			h := handler.NewAuthHandler(userRepo, testJWTSecret, zap.NewNop())

			c, rec := newContext("POST", "/auth/login", tt.input)

			err := h.Login(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, rec.Code)

			body := parseBody(rec)
			fields := body["fields"].(map[string]any)
			assert.Contains(t, fields, tt.field)
		})
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	userRepo := new(mocks.UserRepo)
	h := handler.NewAuthHandler(userRepo, testJWTSecret, zap.NewNop())

	userRepo.On("GetByEmail", mock.Anything, "ghost@test.com").Return(nil, repository.ErrNotFound)

	c, rec := newContext("POST", "/auth/login", map[string]string{
		"email": "ghost@test.com", "password": "password123",
	})

	err := h.Login(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, "invalid credentials", body["error"])
}

func TestLogin_WrongPassword(t *testing.T) {
	userRepo := new(mocks.UserRepo)
	h := handler.NewAuthHandler(userRepo, testJWTSecret, zap.NewNop())

	hashed, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.MinCost)
	userRepo.On("GetByEmail", mock.Anything, "jane@test.com").Return(&model.User{
		ID: "user-1", Password: string(hashed),
	}, nil)

	c, rec := newContext("POST", "/auth/login", map[string]string{
		"email": "jane@test.com", "password": "wrongpassword",
	})

	err := h.Login(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	body := parseBody(rec)
	assert.Equal(t, "invalid credentials", body["error"])
}
