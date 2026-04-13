package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func JWTAuth(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "missing authorization header",
				})
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid authorization header format",
				})
			}

			token, err := jwt.ParseWithClaims(parts[1], &JWTClaims{}, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid or expired token",
				})
			}

			claims, ok := token.Claims.(*JWTClaims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid token claims",
				})
			}

			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)
			return next(c)
		}
	}
}
