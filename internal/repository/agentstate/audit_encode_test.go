package agentstate

import (
	"strings"
	"testing"
)

func TestEncodeAuditMapTruncatesLargePayload(t *testing.T) {
	// ~100KB map value
	big := strings.Repeat("x", 100_000)
	out, err := encodeAuditMap(map[string]any{"blob": big})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"_truncated":true`) {
		t.Fatalf("expected truncation marker, got %s", out)
	}
	small, err := encodeAuditMap(map[string]any{"ok": 1})
	if err != nil {
		t.Fatal(err)
	}
	if small != `{"ok":1}` && !strings.Contains(small, `"ok"`) {
		t.Fatalf("small = %s", small)
	}
}
