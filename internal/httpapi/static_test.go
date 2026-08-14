package httpapi

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

func TestStaticSPAServingReturnsIndexAssetsAndFallback(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, WithStaticFS(spaFixtureFS()))

	cases := []struct {
		path        string
		wantStatus  int
		wantBody    string
		contentType string
	}{
		{path: "/", wantStatus: http.StatusOK, wantBody: `<div id="root"></div>`, contentType: "text/html"},
		{path: "/assets/app.js", wantStatus: http.StatusOK, wantBody: `console.log("fund")`, contentType: "text/javascript"},
		{path: "/funds/019173", wantStatus: http.StatusOK, wantBody: `<div id="root"></div>`, contentType: "text/html"},
	}

	for _, tc := range cases {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if res.Code != tc.wantStatus {
			t.Fatalf("GET %s status = %d, want %d; body=%s", tc.path, res.Code, tc.wantStatus, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), tc.wantBody) {
			t.Fatalf("GET %s body = %q, want %q", tc.path, res.Body.String(), tc.wantBody)
		}
		if got := res.Header().Get("Content-Type"); !strings.Contains(got, tc.contentType) {
			t.Fatalf("GET %s Content-Type = %q, want containing %q", tc.path, got, tc.contentType)
		}
	}
}

func TestStaticSPADoesNotSwallowAPIRoutesAndRejectsTraversal(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, WithStaticFS(spaFixtureFS()))

	api := httptest.NewRecorder()
	router.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/not-found", nil))
	if api.Code != http.StatusNotFound {
		t.Fatalf("GET /api/not-found status = %d, want 404; body=%s", api.Code, api.Body.String())
	}
	if strings.Contains(api.Body.String(), `<div id="root"></div>`) {
		t.Fatalf("API miss should not return SPA index: %s", api.Body.String())
	}

	traversal := httptest.NewRecorder()
	router.ServeHTTP(traversal, httptest.NewRequest(http.MethodGet, "/%2e%2e/secret.txt", nil))
	if traversal.Code != http.StatusBadRequest {
		t.Fatalf("traversal status = %d, want 400; body=%s", traversal.Code, traversal.Body.String())
	}
}

func spaFixtureFS() fs.FS {
	return fstest.MapFS{
		"index.html": {
			Data:    []byte(`<!doctype html><html><body><div id="root"></div><script src="/assets/app.js"></script></body></html>`),
			Mode:    0o644,
			ModTime: time.Date(2026, 7, 7, 11, 30, 0, 0, time.UTC),
		},
		"assets/app.js": {
			Data:    []byte(`console.log("fund")`),
			Mode:    0o644,
			ModTime: time.Date(2026, 7, 7, 11, 31, 0, 0, time.UTC),
		},
		"secret.txt": {
			Data:    []byte(`not served by traversal`),
			Mode:    0o644,
			ModTime: time.Date(2026, 7, 7, 11, 32, 0, 0, time.UTC),
		},
	}
}

func TestStaticRejectsOversizedFile(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, WithStaticFS(&oversizedFS{size: maxStaticFileBytes + 1}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/huge.bin", nil))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type oversizedFS struct{ size int64 }

func (o *oversizedFS) Open(name string) (fs.File, error) {
	if name == "index.html" {
		return &staticMemFile{name: name, data: []byte(`<div id="root"></div>`), size: 20}, nil
	}
	return &staticMemFile{name: name, data: []byte("x"), size: o.size}, nil
}

type staticMemFile struct {
	name string
	data []byte
	size int64
	off  int64
}

func (f *staticMemFile) Stat() (fs.FileInfo, error) {
	return staticMemInfo{name: f.name, size: f.size}, nil
}
func (f *staticMemFile) Read(p []byte) (int, error) {
	if f.off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.off:])
	f.off += int64(n)
	return n, nil
}
func (f *staticMemFile) Close() error { return nil }

type staticMemInfo struct {
	name string
	size int64
}

func (i staticMemInfo) Name() string       { return i.name }
func (i staticMemInfo) Size() int64        { return i.size }
func (i staticMemInfo) Mode() fs.FileMode  { return 0o644 }
func (i staticMemInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (i staticMemInfo) IsDir() bool        { return false }
func (i staticMemInfo) Sys() any           { return nil }
