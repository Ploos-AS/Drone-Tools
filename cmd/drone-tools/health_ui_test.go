package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIndexIncludesFlightHealthUI(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, marker := range []string{"Flight Health", "health-score", "/api/v1/health", "health-findings", "health-algorithm"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("index missing Flight Health marker %q", marker)
		}
	}
}
