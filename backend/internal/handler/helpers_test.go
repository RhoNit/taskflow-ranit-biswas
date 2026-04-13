package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
)

const testJWTSecret = "test-secret"
const testUserID = "user-111"

func newContext(method, path string, body any) (echo.Context, *httptest.ResponseRecorder) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	return c, rec
}

func authedContext(method, path string, body any) (echo.Context, *httptest.ResponseRecorder) {
	c, rec := newContext(method, path, body)
	c.Set("user_id", testUserID)
	return c, rec
}

func parseBody(rec *httptest.ResponseRecorder) map[string]any {
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	return m
}
