package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const userKey ctxKey = "user"

// User is who is calling, taken from the JWT.
type User struct {
	ID   string
	Role string // viewer, sizing_engineer or admin
}

func userFrom(r *http.Request) User {
	u, _ := r.Context().Value(userKey).(User)
	return u
}

// Auth checks the "Authorization: Bearer <token>" header on every request.
func Auth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "missing bearer token")
				return
			}
			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(tokenStr, claims,
				func(t *jwt.Token) (any, error) { return secret, nil },
				jwt.WithValidMethods([]string{"HS256"}), // reject "alg: none" tricks
				jwt.WithExpirationRequired(),
			)
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			sub, _ := claims.GetSubject()
			role, _ := claims["role"].(string)
			ctx := context.WithValue(r.Context(), userKey, User{ID: sub, Role: role})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole only lets listed roles through.
func RequireRole(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains(roles, userFrom(r).Role) {
			writeError(w, r, http.StatusForbidden, "your role cannot do this")
			return
		}
		next(w, r)
	}
}
