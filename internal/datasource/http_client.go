package datasource

import (
	"net/http"
	"time"
)

// Shared Yahoo clients reuse the transport connection pool across refreshes
// instead of allocating a new http.Client (and transport) per request.
// Each caller keeps its own timeout budget: UI-bounded index quotes use 8s,
// while stock snapshots and history crawls tolerate slower upstreams at 12s.
var (
	yahooQuoteClient   = &http.Client{Timeout: 8 * time.Second}
	yahooHistoryClient = &http.Client{Timeout: 12 * time.Second}
)
