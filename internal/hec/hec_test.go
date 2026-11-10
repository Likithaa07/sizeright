package hec

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriterSendsToSplunk(t *testing.T) {
	got := make(chan string, 1)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Splunk test-token" {
			t.Errorf("bad auth header %q", r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		got <- string(b)
	}))
	defer fake.Close()

	w := New(fake.URL, "test-token", false)
	w.Write([]byte(`{"msg":"hello"}` + "\n"))

	select {
	case body := <-got:
		if !strings.Contains(body, `"event":{"msg":"hello"}`) {
			t.Errorf("unexpected body %s", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nothing sent to Splunk")
	}
}
