package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

type stubHoldings struct {
	codeCalls int
	allCalls  int
}

func (s *stubHoldings) CrawlCode(ctx context.Context, code string) (int, string, error) {
	s.codeCalls++
	return 3, "2026-03-31", nil
}

func (s *stubHoldings) CrawlAllHeld(ctx context.Context) (int, int, error) {
	s.allCalls++
	return 2, 7, nil
}

func TestAdminCrawlHoldingsRoutes(t *testing.T) {
	stub := &stubHoldings{}
	router := NewRouter(config.Config{AdminKey: "test-key"}, WithHoldingsCrawler(stub))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-holdings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/crawl-holdings", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("held status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "complete" || body["mode"] != "held" || body["added"].(float64) != 7 {
		t.Fatalf("body=%v", body)
	}
	if stub.allCalls != 1 {
		t.Fatalf("allCalls=%d", stub.allCalls)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/crawl-holdings?code=019173", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("single status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["mode"] != "single" || body["fund_code"] != "019173" || body["report_date"] != "2026-03-31" {
		t.Fatalf("single body=%v", body)
	}
	if stub.codeCalls != 1 {
		t.Fatalf("codeCalls=%d", stub.codeCalls)
	}
}

type stubSnapshots struct {
	codeCalls int
	allCalls  int
}

func (s *stubSnapshots) RecalcCode(ctx context.Context, code string) error {
	s.codeCalls++
	return nil
}

func (s *stubSnapshots) RecalcAll(ctx context.Context) (int, []string, error) {
	s.allCalls++
	return 5, nil, nil
}

func TestAdminRecalculateSnapshotRoutes(t *testing.T) {
	stub := &stubSnapshots{}
	router := NewRouter(config.Config{AdminKey: "test-key"}, WithSnapshotRecalculator(stub))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/recalculate-snapshot", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/recalculate-snapshot", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("all status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "complete" || body["mode"] != "all" || body["codes"].(float64) != 5 {
		t.Fatalf("body=%v", body)
	}
	fc, ok := body["failed_codes"].([]any)
	if !ok || len(fc) != 0 {
		t.Fatalf("failed_codes want empty array, got %v", body["failed_codes"])
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/recalculate-snapshot?code=019173", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("single status=%d", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["mode"] != "single" || body["fund_code"] != "019173" {
		t.Fatalf("single body=%v", body)
	}
	if stub.codeCalls != 1 || stub.allCalls != 1 {
		t.Fatalf("calls code=%d all=%d", stub.codeCalls, stub.allCalls)
	}
}

type stubNav struct {
	codeCalls int
	allCalls  int
}

func (s *stubNav) CrawlCode(ctx context.Context, code string) (int, string, error) {
	s.codeCalls++
	return 12, "2026-07-14", nil
}

func (s *stubNav) CrawlAllHeld(ctx context.Context) (int, int, error) {
	s.allCalls++
	return 4, 20, nil
}

func TestAdminCrawlNavRoutes(t *testing.T) {
	stub := &stubNav{}
	router := NewRouter(config.Config{AdminKey: "test-key"}, WithNavCrawler(stub))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/crawl-nav", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/crawl-nav", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("held status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "complete" || body["mode"] != "held" || body["securities"].(float64) != 4 {
		t.Fatalf("body=%v", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/crawl-nav?code=019173", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("single status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["mode"] != "single" || body["fund_code"] != "019173" || body["latest"] != "2026-07-14" {
		t.Fatalf("single body=%v", body)
	}
	if stub.codeCalls != 1 || stub.allCalls != 1 {
		t.Fatalf("calls code=%d all=%d", stub.codeCalls, stub.allCalls)
	}

	// stale_only without DB admin service: still must not call CrawlAllHeld.
	req = httptest.NewRequest(http.MethodPost, "/api/admin/crawl-nav?stale_only=1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("stale_only no-admin status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["mode"] != "stale_only" {
		t.Fatalf("stale_only body=%v", body)
	}
	if stub.allCalls != 1 {
		t.Fatalf("stale_only must not call CrawlAllHeld; allCalls=%d", stub.allCalls)
	}
}

type stubSnapshotsPartial struct {
	stubSnapshots
}

func (s *stubSnapshotsPartial) RecalcAll(ctx context.Context) (int, []string, error) {
	s.allCalls++
	return 2, []string{"BAD1", "BAD2"}, nil
}

func TestAdminRecalculateSnapshotPartial(t *testing.T) {
	stub := &stubSnapshotsPartial{}
	router := NewRouter(config.Config{AdminKey: "test-key"}, WithSnapshotRecalculator(stub))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/recalculate-snapshot", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("partial status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "partial" || body["mode"] != "all" || body["codes"].(float64) != 2 {
		t.Fatalf("body=%v", body)
	}
	fc, ok := body["failed_codes"].([]any)
	if !ok || len(fc) != 2 || fc[0] != "BAD1" {
		t.Fatalf("failed_codes=%v", body["failed_codes"])
	}
}

type stubSnapshotsAllFailed struct {
	stubSnapshots
}

func (s *stubSnapshotsAllFailed) RecalcAll(ctx context.Context) (int, []string, error) {
	s.allCalls++
	return 0, []string{"X"}, nil
}

func TestAdminRecalculateSnapshotAllFailed(t *testing.T) {
	stub := &stubSnapshotsAllFailed{}
	router := NewRouter(config.Config{AdminKey: "test-key"}, WithSnapshotRecalculator(stub))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/recalculate-snapshot", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("all-failed status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "error" {
		t.Fatalf("body=%v", body)
	}
	if body["failed_codes"] == nil {
		t.Fatalf("expected failed_codes: %v", body)
	}
}
