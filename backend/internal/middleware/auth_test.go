package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/ranit-biswas/taskflow/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret"

func makeToken(userID, email, secret string, expiresAt time.Time) string {
	claims := &middleware.JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

func callMiddleware(authHeader string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := middleware.JWTAuth(testSecret)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	handler(c)

	return rec, c
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	rec, _ := callMiddleware("")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	rec, _ := callMiddleware("Token abc123")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTAuth_NoBearerPrefix(t *testing.T) {
	rec, _ := callMiddleware("just-a-token-string")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	rec, _ := callMiddleware("Bearer totally.invalid.token")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	token := makeToken("user-1", "a@b.com", testSecret, time.Now().Add(-1*time.Hour))
	rec, _ := callMiddleware("Bearer " + token)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTAuth_WrongSigningKey(t *testing.T) {
	token := makeToken("user-1", "a@b.com", "wrong-secret", time.Now().Add(time.Hour))
	rec, _ := callMiddleware("Bearer " + token)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTAuth_ValidToken(t *testing.T) {
	token := makeToken("user-42", "jane@test.com", testSecret, time.Now().Add(time.Hour))
	rec, c := callMiddleware("Bearer " + token)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-42", c.Get("user_id"))
	assert.Equal(t, "jane@test.com", c.Get("email"))
}
