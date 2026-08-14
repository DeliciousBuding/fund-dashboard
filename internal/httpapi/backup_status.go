package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/go-chi/chi/v5"
)

type backupStatusResponse struct {
	Status                string         `json:"status"`
	Path                  string         `json:"path"`
	Latest                *string        `json:"latest"`
	LatestAgeHours        *float64       `json:"latest_age_hours"`
	Count                 map[string]int `json:"count"`
	BackupProducerEnabled bool           `json:"backup_producer_enabled"`
	DecisionBoundary      string         `json:"decision_boundary"`
}

func registerBackupStatusRoutes(r chi.Router, cfg config.Config) {
	r.Get("/backup-status", func(w http.ResponseWriter, req *http.Request) {
		WriteJSON(w, http.StatusOK, readBackupStatus(cfg.BackupDir, time.Now()))
	})
}

func readBackupStatus(backupDir string, now time.Time) backupStatusResponse {
	response := backupStatusResponse{
		Status:                "no_backups",
		Path:                  backupDir,
		Count:                 map[string]int{"daily": 0, "weekly": 0, "monthly": 0, "manifests": 0},
		BackupProducerEnabled: false,
		DecisionBoundary:      "read_only",
	}
	if strings.TrimSpace(backupDir) == "" {
		return response
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return response
	}

	daily := backupFiles(entries, "")
	response.Count["daily"] = len(daily)
	response.Count["manifests"] = countMatching(entries, "manifest_", ".json")
	response.Count["weekly"] = countBackupFiles(filepath.Join(backupDir, "weekly"))
	response.Count["monthly"] = countBackupFiles(filepath.Join(backupDir, "monthly"))

	if len(daily) == 0 {
		return response
	}
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].modTime.After(daily[j].modTime)
	})
	response.Status = "ok"
	response.Latest = &daily[0].name
	age := now.Sub(daily[0].modTime).Hours()
	if age < 0 {
		age = 0
	}
	rounded := float64(int(age*10+0.5)) / 10
	response.LatestAgeHours = &rounded
	return response
}

type backupFile struct {
	name    string
	modTime time.Time
}

func backupFiles(entries []os.DirEntry, prefix string) []backupFile {
	files := make([]backupFile, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "fund_") || !strings.HasSuffix(entry.Name(), ".db.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, backupFile{name: prefix + entry.Name(), modTime: info.ModTime()})
	}
	return files
}

func countBackupFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	return len(backupFiles(entries, ""))
}

func countMatching(entries []os.DirEntry, prefix string, suffix string) int {
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), suffix) {
			count++
		}
	}
	return count
}
