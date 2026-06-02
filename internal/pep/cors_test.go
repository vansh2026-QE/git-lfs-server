package pep_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/pep"
)

func corsHandler() http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return pep.CORS([]string{"chrome-extension://abc"}, inner)
}

func TestCORSPreflightAllowedOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/demo/content", nil)
	req.Header.Set("Origin", "chrome-extension://abc")
	rec := httptest.NewRecorder()
	corsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "chrome-extension://abc" {
		t.Errorf("Allow-Origin = %q, want the request origin", got)
	}
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected Access-Control-Allow-Headers on preflight")
	}
}

func TestCORSPreflightDisallowedOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/demo/content", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	corsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty for disallowed origin", got)
	}
}

func TestCORSActualRequestPassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/demo/content", nil)
	req.Header.Set("Origin", "chrome-extension://abc")
	rec := httptest.NewRecorder()
	corsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (handler reached)", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "chrome-extension://abc" {
		t.Errorf("Allow-Origin = %q, want the request origin", got)
	}
}
