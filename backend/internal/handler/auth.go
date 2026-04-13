package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ranit-biswas/taskflow/internal/middleware"
	"github.com/ranit-biswas/taskflow/internal/model"
	"github.com/ranit-biswas/taskflow/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	users     repository.UserRepository
	jwtSecret string
	logger    *zap.Logger
}

func NewAuthHandler(users repository.UserRepository, jwtSecret string, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{users: users, jwtSecret: jwtSecret, logger: logger}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	fieldErrors := make(map[string]string)
	if req.Name == "" {
		fieldErrors["name"] = "is required"
	}
	if req.Email == "" {
		fieldErrors["email"] = "is required"
	}
	if req.Password == "" {
		fieldErrors["password"] = "is required"
	} else if len(req.Password) < 6 {
		fieldErrors["password"] = "must be at least 6 characters"
	}
	if len(fieldErrors) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  "validation failed",
			"fields": fieldErrors,
		})
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		h.logger.Error("hashing password", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	user := &model.User{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Email:     req.Email,
		Password:  string(hashed),
		CreatedAt: time.Now().UTC(),
	}

	if err := h.users.Create(c.Request().Context(), user); err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "validation failed",
				"fields": map[string]string{"email": "already exists"},
			})
		}
		h.logger.Error("creating user", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	h.logger.Info("user registered", zap.String("user_id", user.ID), zap.String("email", user.Email))
	return c.JSON(http.StatusCreated, user)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	fieldErrors := make(map[string]string)
	if req.Email == "" {
		fieldErrors["email"] = "is required"
	}
	if req.Password == "" {
		fieldErrors["password"] = "is required"
	}
	if len(fieldErrors) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  "validation failed",
			"fields": fieldErrors,
		})
	}

	user, err := h.users.GetByEmail(c.Request().Context(), req.Email)
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}
	if err != nil {
		h.logger.Error("fetching user", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	claims := &middleware.JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		h.logger.Error("signing token", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	h.logger.Info("user logged in", zap.String("user_id", user.ID))
	return c.JSON(http.StatusOK, map[string]string{"token": signed})
}
