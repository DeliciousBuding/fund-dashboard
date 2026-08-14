package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupStatusReportsNoBackupsForMissingOrEmptyDirectory(t *testing.T) {
	cfg := testCfg()
	cfg.BackupDir = filepath.Join(t.TempDir(), "missing")
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/backup-status", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminKey)

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", res.Code, res.Body.String())
	}
	for _, want := range []string{`"status":"no_backups"`, `"latest":null`, `"daily":0`, `"backup_producer_enabled":false`} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, res.Body.String())
		}
	}
}

func TestBackupStatusListsExistingBackupFilesReadOnly(t *testing.T) {
	backupDir := t.TempDir()
	writeBackupFixture(t, backupDir, "fund_20260707_100000.db.gz")
	writeBackupFixture(t, filepath.Join(backupDir, "weekly"), "fund_20260701_000000.db.gz")
	writeBackupFixture(t, filepath.Join(backupDir, "monthly"), "fund_20260701_000000.db.gz")
	if err := os.WriteFile(filepath.Join(backupDir, "manifest_20260707.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "ignore.txt"), []byte(`not a backup`), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}

	cfg := testCfg()
	cfg.BackupDir = backupDir
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/backup-status", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminKey)

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", res.Code, res.Body.String())
	}
	for _, want := range []string{`"status":"ok"`, `"latest":"fund_20260707_100000.db.gz"`, `"daily":1`, `"weekly":1`, `"monthly":1`, `"manifests":1`} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, res.Body.String())
		}
	}

	post := httptest.NewRecorder()
	postReq := httptest.NewRequest(http.MethodPost, "/api/admin/backup-status", nil)
	postReq.Header.Set("Authorization", "Bearer "+testAdminKey)
	router.ServeHTTP(post, postReq)
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405; body=%s", post.Code, post.Body.String())
	}
}

func writeBackupFixture(t *testing.T, dir string, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(`backup`), 0o644); err != nil {
		t.Fatalf("write backup %s: %v", name, err)
	}
}
