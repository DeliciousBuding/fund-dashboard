package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestClientErrorMessageHidesSQL(t *testing.T) {
	msg := clientErrorMessage(http.StatusInternalServerError, errors.New(`pq: syntax error at or near "PRAGMA"`))
	if msg != "internal_error" {
		t.Fatalf("got %q", msg)
	}
	msg = clientErrorMessage(http.StatusBadGateway, errors.New("Get \"https://x\": dial tcp: i/o timeout"))
	if msg != "upstream_unavailable" {
		t.Fatalf("got %q", msg)
	}
	msg = clientErrorMessage(http.StatusBadRequest, errors.New("confirm_share must be positive for buy/sell"))
	if !strings.Contains(msg, "confirm_share") {
		t.Fatalf("safe validation should pass through, got %q", msg)
	}
}
