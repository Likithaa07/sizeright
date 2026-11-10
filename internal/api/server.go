package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Likithaa07/sizeright/internal/cache"
	"github.com/Likithaa07/sizeright/internal/sizing"
	"github.com/Likithaa07/sizeright/internal/store"
)

// Server holds everything the handlers need.
type Server struct {
	Rules     sizing.Rules
	Store     *store.Store
	JWTSecret []byte
	Log       *slog.Logger
	Cache     *cache.Cache // optional: nil means no cache
	RateLimit int          // requests per minute per user
}

// Routes wires URLs to handlers and wraps them in middleware.
func (s *Server) Routes() http.Handler {
	authed := http.NewServeMux()
	authed.HandleFunc("POST /v1/sizings", RequireRole(s.createSizing, "sizing_engineer", "admin"))
	authed.HandleFunc("GET /v1/sizings/{id}", RequireRole(s.getSizing, "viewer", "sizing_engineer", "admin"))
	authed.HandleFunc("GET /v1/sizings/{id}/terraform", RequireRole(s.getTerraform, "sizing_engineer", "admin"))

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.HandleFunc("GET /readyz", s.ready)
	root.Handle("/v1/", Auth(s.JWTSecret)(RateLimit(s.RateLimit)(authed)))

	return RequestID(AccessLog(s.Log)(Recover(s.Log)(root)))
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
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// Cache key ignores customer_id: the answer only depends on the workload.
	keyInput := sub
	keyInput.CustomerID = ""
	key := cache.Key(s.Rules.Version, keyInput)
	var res sizing.Result
	cached, hit := s.Cache.Get(ctx, key)
	if !hit || json.Unmarshal(cached, &res) != nil {
		hit = false
		res = sizing.Size(sub, s.Rules)
		b, _ := json.Marshal(res)
		s.Cache.Set(ctx, key, b)
	}

	reqJSON, _ := json.Marshal(sub)
	resJSON, _ := json.Marshal(res)
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
		"region", sub.Region, "indexers", res.Indexers, "cache_hit", hit, "request_id", requestID(r))
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "result": res})
}

// loadOwned loads a sizing and applies the ownership check. It writes the
// error response itself and returns ok=false if the caller should stop.
func (s *Server) loadOwned(w http.ResponseWriter, r *http.Request) (store.Sizing, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "id must be a number")
		return store.Sizing{}, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	sz, err := s.Store.Get(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "sizing not found")
		return sz, false
	}
	if err != nil {
		s.Log.Error("get sizing failed", "err", err, "request_id", requestID(r))
		writeError(w, r, http.StatusInternalServerError, "could not load sizing")
		return sz, false
	}
	// Ownership check (stops IDOR): only the creator or an admin may read it.
	// Return 404, not 403, so attackers can't tell which IDs exist.
	u := userFrom(r)
	if sz.CreatedBy != u.ID && u.Role != "admin" {
		writeError(w, r, http.StatusNotFound, "sizing not found")
		return sz, false
	}
	return sz, true
}

func (s *Server) getSizing(w http.ResponseWriter, r *http.Request) {
	if sz, ok := s.loadOwned(w, r); ok {
		writeJSON(w, http.StatusOK, sz)
	}
}

// getTerraform returns deployment inputs that the terraform/ module reads.
func (s *Server) getTerraform(w http.ResponseWriter, r *http.Request) {
	sz, ok := s.loadOwned(w, r)
	if !ok {
		return
	}
	var sub sizing.Subscription
	var res sizing.Result
	if json.Unmarshal(sz.Request, &sub) != nil || json.Unmarshal(sz.Result, &res) != nil {
		writeError(w, r, http.StatusInternalServerError, "stored sizing is corrupt")
		return
	}
	tfvars := map[string]any{
		"customer_id":               sub.CustomerID,
		"region":                    sub.Region,
		"indexer_count":             res.Indexers,
		"indexer_instance_type":     "i4i.4xlarge", // example choice; move to a catalog file later
		"search_head_count":         res.CoreSearchHeads + res.PremiumSearchHeads,
		"search_head_instance_type": "c6i.4xlarge",
		"local_storage_tb":          res.LocalStorageTB,
		"smartstore":                sub.SmartStore,
	}
	w.Header().Set("Content-Disposition", `attachment; filename="terraform.tfvars.json"`)
	writeJSON(w, http.StatusOK, tfvars)
}
