package api

import (
	"encoding/json"
	"net/http"
)

// writeJSON sends any value as JSON with a status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError sends a consistent error body: {"error": "...", "request_id": "..."}.
func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "request_id": requestID(r)})
}
