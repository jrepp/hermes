package otel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
)

func TestPromQLQueryProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "up" {
			t.Fatalf("unexpected query: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer upstream.Close()

	handler := Handler(&config.OpenTelemetry{PrometheusURL: upstream.URL}, hclog.NewNullLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/v2/otel/promql/query?query=up", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if body["status"] != "success" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestPromQLQueryRequiresPrometheusURL(t *testing.T) {
	handler := Handler(&config.OpenTelemetry{}, hclog.NewNullLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/v2/otel/promql/query?query=up", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}
