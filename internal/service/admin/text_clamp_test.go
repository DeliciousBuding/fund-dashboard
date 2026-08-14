package admin

import (
	"strings"
	"testing"
)

func TestClampAdminTextRuneSafe(t *testing.T) {
	if clampAdminText("  ab  ", 10) != "ab" {
		t.Fatal("trim")
	}
	long := strings.Repeat("测", 100)
	got := clampAdminText(long, 10)
	if len([]rune(got)) != 10 {
		t.Fatalf("rune len=%d want 10", len([]rune(got)))
	}
	if clampAdminText("short", 32) != "short" {
		t.Fatal("passthrough")
	}
}

func TestSystemAnomalyFieldLimits(t *testing.T) {
	// Document accepted limits used by querySystemAnomalies (#244).
	const (
		maxCode    = 32
		maxDir     = 32
		maxTrade   = 40
		maxAnomaly = 500
	)
	code := clampAdminText(strings.Repeat("C", 100), maxCode)
	if len([]rune(code)) != maxCode {
		t.Fatalf("code len=%d", len([]rune(code)))
	}
	anom := clampAdminText(strings.Repeat("A", 1000), maxAnomaly)
	if len([]rune(anom)) != maxAnomaly {
		t.Fatalf("anomaly len=%d", len([]rune(anom)))
	}
	dir := clampAdminText(strings.Repeat("D", 80), maxDir)
	if len([]rune(dir)) != maxDir {
		t.Fatalf("direction len=%d", len([]rune(dir)))
	}
	tt := clampAdminText(strings.Repeat("T", 80), maxTrade)
	if len([]rune(tt)) != maxTrade {
		t.Fatalf("trade_time len=%d", len([]rune(tt)))
	}
}

func TestFreshnessFieldLimits(t *testing.T) {
	const (
		maxCode = 32
		maxName = 200
		maxType = 32
		maxNAV  = 40
	)
	if n := len([]rune(clampAdminText(strings.Repeat("N", 500), maxName))); n != maxName {
		t.Fatalf("name len=%d", n)
	}
	if n := len([]rune(clampAdminText(strings.Repeat("X", 100), maxCode))); n != maxCode {
		t.Fatalf("code len=%d", n)
	}
	if n := len([]rune(clampAdminText(strings.Repeat("Y", 100), maxType))); n != maxType {
		t.Fatalf("type len=%d", n)
	}
	if n := len([]rune(clampAdminText(strings.Repeat("Z", 100), maxNAV))); n != maxNAV {
		t.Fatalf("last_nav len=%d", n)
	}
}

func TestAlertFieldLimits(t *testing.T) {
	msg := clampAdminText(strings.Repeat("M", 1000), 500)
	if len([]rune(msg)) != 500 {
		t.Fatalf("message len=%d", len([]rune(msg)))
	}
	if n := len([]rune(clampAdminText(strings.Repeat("N", 300), 200))); n != 200 {
		t.Fatalf("name len=%d", n)
	}
}

func TestHoldingsCoverageFieldLimits(t *testing.T) {
	if n := len([]rune(clampAdminText(strings.Repeat("T", 100), 64))); n != 64 {
		t.Fatalf("fund_type len=%d", n)
	}
}
