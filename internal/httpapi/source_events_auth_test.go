package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSourceEventWritesRequireEdgeAuth(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db), WithDBDriver("sqlite"))

	body, _ := json.Marshal(map[string]any{"title": "probe", "source": "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/portfolio/source-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("POST without edge key status=%d want 401 body=%s", res.Code, res.Body.String())
	}

	// with edge key should succeed
	created := doJSONRequest(t, router, http.MethodPost, "/api/portfolio/source-events", map[string]any{
		"title": "edge-ok", "source": "test",
	}, http.StatusCreated)
	id, _ := created["id"].(float64)
	if id <= 0 {
		t.Fatalf("created=%#v want id", created)
	}
}
