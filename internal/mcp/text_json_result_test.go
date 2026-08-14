package mcp

import (
	"strings"
	"testing"
)

func TestTextJSONResultCapsSize(t *testing.T) {
	// Build a payload that encodes over 1 MiB when pretty-printed.
	big := strings.Repeat("x", maxToolResultBytes)
	payload := map[string]any{"blob": big}
	_, err := textJSONResult(payload)
	if err == nil {
		t.Fatal("expected tool_result_too_large")
	}
	if err.Message != "tool_result_too_large" {
		t.Fatalf("message=%q", err.Message)
	}
}

func TestTextJSONResultSmallOK(t *testing.T) {
	out, err := textJSONResult(map[string]any{"ok": true})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	content, _ := out["content"].([]map[string]any)
	if len(content) != 1 {
		t.Fatalf("content=%v", out)
	}
}
