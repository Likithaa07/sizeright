package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Likithaa07/sizeright/internal/sizing"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-that-is-at-least-32-chars!!"

func token(t *testing.T, user, role string, exp time.Duration) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user, "role": role, "exp": time.Now().Add(exp).Unix(),
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAuthAndStatusCodes(t *testing.T) {
	rules, err := sizing.LoadRules("../../rules/v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Store is nil: these cases must all be rejected before the database is used.
	srv := (&Server{Rules: rules, JWTSecret: []byte(testSecret),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}).Routes()

	tests := []struct {
		name, auth, body string
		want             int
	}{
		{"no token", "", `{}`, http.StatusUnauthorized},
		{"expired token", "Bearer " + token(t, "alice", "sizing_engineer", -time.Minute), `{}`, http.StatusUnauthorized},
		{"wrong role", "Bearer " + token(t, "bob", "viewer", time.Hour), `{}`, http.StatusForbidden},
		{"bad JSON", "Bearer " + token(t, "alice", "sizing_engineer", time.Hour), `{not json`, http.StatusBadRequest},
		{"unknown field", "Bearer " + token(t, "alice", "sizing_engineer", time.Hour), `{"hacker":true}`, http.StatusBadRequest},
		{"invalid values", "Bearer " + token(t, "alice", "sizing_engineer", time.Hour), `{"ingest_gb_per_day":-1}`, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/sizings", strings.NewReader(tt.body))
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.want, rec.Body)
			}
			if rec.Header().Get("X-Request-ID") == "" {
				t.Error("missing X-Request-ID header")
			}
		})
	}
}
