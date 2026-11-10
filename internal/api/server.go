package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Likithaa07/sizeright/internal/sizing"
	"github.com/Likithaa07/sizeright/internal/store"
)

// Server holds everything the handlers need.
type Server struct {
	Rules     sizing.Rules
	Store     *store.Store
	JWTSecret []byte
	Log       *slog.Logger
}

// Routes wires URLs to handlers and wraps them in middleware.
func (s *Server) Routes() http.Handler {
	authed := http.NewServeMux()
	authed.HandleFunc("POST /v1/sizings", RequireRole(s.createSizing, "sizing_engineer", "admin"))
	authed.HandleFunc("GET /v1/sizings/{id}", RequireRole(s.getSizing, "viewer", "sizing_engineer", "admin"))

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.HandleFunc("GET /readyz", s.ready)
	root.Handle("/v1/", Auth(s.JWTSecret)(authed))

	return RequestID(Recover(s.Log)(root))
}

// ready returns 503 if the database is down, so the load balancer stops sending traffic.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) createSizing(w http.ResponseWriter, r *http.Request) {
	var sub sizing.Subscription
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sub); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if errs := sub.Validate(s.Rules); len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": errs, "request_id": requestID(r)})
		return
	}
	res := sizing.Size(sub, s.Rules)

	reqJSON, _ := json.Marshal(sub)
	resJSON, _ := json.Marshal(res)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	id, err := s.Store.Save(ctx, store.Sizing{
		CustomerID: sub.CustomerID, Request: reqJSON, Result: resJSON,
		RulesVersion: res.RulesVersion, CreatedBy: userFrom(r).ID,
	})
	if err != nil {
		s.Log.Error("save sizing failed", "err", err, "request_id", requestID(r))
		writeError(w, r, http.StatusInternalServerError, "could not save sizing")
		return
	}
	s.Log.Info("sizing created", "sizing_id", id, "customer_id", sub.CustomerID,
		"region", sub.Region, "indexers", res.Indexers, "request_id", requestID(r))
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "result": res})
}

func (s *Server) getSizing(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "id must be a number")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	sz, err := s.Store.Get(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "sizing not found")
		return
	}
	if err != nil {
		s.Log.Error("get sizing failed", "err", err, "request_id", requestID(r))
		writeError(w, r, http.StatusInternalServerError, "could not load sizing")
		return
	}
	// Ownership check (stops IDOR): only the creator or an admin may read it.
	// Return 404, not 403, so attackers can't tell which IDs exist.
	u := userFrom(r)
	if sz.CreatedBy != u.ID && u.Role != "admin" {
		writeError(w, r, http.StatusNotFound, "sizing not found")
		return
	}
	writeJSON(w, http.StatusOK, sz)
}
