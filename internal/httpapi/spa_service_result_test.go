package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// TestWriteServiceResultMapsValidationVsInternal pins the 400-vs-500 split:
// ValidationError is a client-input failure; everything else (including DB
// faults) must surface as 500 and never as a client 400.
func TestWriteServiceResultMapsValidationVsInternal(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(""))
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{"validation", portfoliosvc.NewValidationError("amount max 1000000"), http.StatusBadRequest, "amount max 1000000"},
		{"wrapped validation", fmt.Errorf("context: %w", portfoliosvc.NewValidationError("fund_code is required")), http.StatusBadRequest, "context: fund_code is required"},
		{"db failure", errors.New("insert dca plan: sql: database is closed"), http.StatusInternalServerError, "internal_error"},
		{"query failure", errors.New("query dca plans: sql: no such table: dca_plans"), http.StatusInternalServerError, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeServiceResult(rec, req, map[string]any{"ok": true}, tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			if body["error"] != tc.wantError {
				t.Fatalf("error = %q, want %q", body["error"], tc.wantError)
			}
		})
	}
}

func TestWriteServiceResultSuccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/write", nil)
	rec := httptest.NewRecorder()
	writeServiceResult(rec, req, map[string]any{"ok": true}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("body = %s, want ok=true", rec.Body.String())
	}
}

// TestSPAWriteExtensionDBFailureIs500 exercises the real handler with a closed
// DB: a valid request that hits the database must come back 500 internal_error,
// not 400 (the pre-fix behavior mapped every service error to 400).
func TestSPAWriteExtensionDBFailureIs500(t *testing.T) {
	rawDB := openSPAExtensionFixture(t)
	svc := portfoliosvc.NewService(rawDB)
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	handler := handleUpsertDCAPlan(&svc)
	req := httptest.NewRequest(http.MethodPost, "/api/dca/plans", strings.NewReader(`{"fund_code":"AAPL","amount":100}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["error"] != "internal_error" {
		t.Fatalf("error = %q, want %q", body["error"], "internal_error")
	}
}
