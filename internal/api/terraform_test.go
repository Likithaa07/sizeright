package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Likithaa07/sizeright/internal/sizing"
	"github.com/Likithaa07/sizeright/internal/store"
)

func newTestServer(t *testing.T, st *store.Store) http.Handler {
	t.Helper()
	rules, err := sizing.LoadRules("../../rules/v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return (&Server{Rules: rules, Store: st, JWTSecret: []byte(testSecret),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), RateLimit: 100}).Routes()
}

func do(srv http.Handler, method, path, tok, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// Runs in CI: these must be rejected before the database is touched.
func TestTerraformRejections(t *testing.T) {
	srv := newTestServer(t, nil)
	tests := []struct {
		name, tok, path string
		want            int
	}{
		{"no token", "", "/v1/sizings/1/terraform", http.StatusUnauthorized},
		{"viewer can't export", token(t, "vic", "viewer", time.Hour), "/v1/sizings/1/terraform", http.StatusForbidden},
		{"id not a number", token(t, "alice", "sizing_engineer", time.Hour), "/v1/sizings/abc/terraform", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := do(srv, http.MethodGet, tt.path, tt.tok, ""); rec.Code != tt.want {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

// Full flow against a real Postgres. Skipped unless DATABASE_URL is set.
func TestTerraformEndToEnd(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	st, err := store.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := newTestServer(t, st)
	alice := token(t, "alice", "sizing_engineer", time.Hour)
	bob := token(t, "bob", "sizing_engineer", time.Hour)

	body := `{"customer_id":"tftest","cloud":"aws","region":"ap-southeast-2","ingest_gb_per_day":500,
	  "retention_days":90,"search_profile":"moderate","concurrent_users":40,"premium_apps":["enterprise_security"],
	  "smartstore":true,"high_availability":true}`
	rec := do(srv, http.MethodPost, "/v1/sizings", alice, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("no id in create response: %s", rec.Body)
	}
	path := "/v1/sizings/" + itoa(created.ID) + "/terraform"

	rec = do(srv, http.MethodGet, path, alice, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("owner: status %d, body %s", rec.Code, rec.Body)
	}
	var tf map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &tf); err != nil {
		t.Fatal(err)
	}
	if tf["region"] != "ap-southeast-2" || tf["indexer_count"].(float64) < 3 {
		t.Errorf("unexpected tfvars: %v", tf)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "terraform.tfvars.json") {
		t.Error("missing download filename")
	}

	// IDOR check: another engineer must not see Alice's sizing.
	if rec := do(srv, http.MethodGet, path, bob, ""); rec.Code != http.StatusNotFound {
		t.Errorf("non-owner: status %d, want 404", rec.Code)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
