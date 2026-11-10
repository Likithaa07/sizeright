package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

const requestIDKey ctxKey = "request_id"

func requestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey).(string)
	return id
}

// RequestID gives every request an ID and returns it in the X-Request-ID header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// Recover turns a panic into a 500 instead of crashing the whole server.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic", "panic", rec, "request_id", requestID(r))
					writeError(w, r, http.StatusInternalServerError, "internal error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
