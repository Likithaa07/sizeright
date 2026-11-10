package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Likithaa07/sizeright/internal/sizing"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func main() {
	rules, err := sizing.LoadRules("rules/v1.yaml")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /v1/sizings", func(w http.ResponseWriter, r *http.Request) {
		var sub sizing.Subscription
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)) // max 1 MB
		dec.DisallowUnknownFields()
		if err := dec.Decode(&sub); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		if errs := sub.Validate(rules); len(errs) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": errs})
			return
		}
		writeJSON(w, http.StatusCreated, sizing.Size(sub, rules))
	})

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second, // timeouts protect against slow clients
		WriteTimeout: 10 * time.Second,
	}
	log.Println("listening on :8080")
	log.Fatal(srv.ListenAndServe())
}
