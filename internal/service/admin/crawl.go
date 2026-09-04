package admin

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// ── crawl / price-refresh convergence ───────────────────────────────────────
//
// Stale-code recommendation, the per-code refresh batch loop and the
// complete/partial/error status rule used to be implemented three times: once
// for the MCP read tools, once for the admin REST crawl endpoint and once for
// the scheduled job. AGENTS.md forbids that ("REST 和 MCP 逻辑重复 → 都调用
// services"), so this file is the single implementation and all three call
// sites are thin protocol adapters over it.
//
// Dependency direction stays unidirectional: this package owns the port
// (CodeRefresher) and never imports internal/jobs, internal/mcp or
// internal/httpapi. Each caller adapts its own crawler to the port, so the
// batch semantics can no longer drift between protocols.

// CodeRefresher refreshes one security code and reports how many rows it added.
// Callers adapt their own crawler to this one-function port: the job-layer
// price refresher, the MCP NavCrawler and the admin REST crawler all reduce to
// it, which is what lets the batch below have exactly one implementation.
type CodeRefresher func(ctx context.Context, code string) (added int, err error)

// defaultBatchFailureLogMessage is used when a caller does not name its own
// per-code failure log message. A failure is never swallowed silently.
const defaultBatchFailureLogMessage = "crawl code batch: code failed"

// RecommendedRefreshCodes merges held stale + missing NAV codes into the
// canonical refresh list for a stale-only crawl: normalized, de-duplicated,
// stale first then missing, input order preserved within each group.
//
// This is the single source of truth for "what should be refreshed". It was
// previously duplicated verbatim as mcp.RecommendedRefreshCodes (#252) and
// jobs.heldRefreshCodes, which is how the MCP tool, the admin endpoint and the
// scheduled job could silently disagree about the same maintenance decision.
func RecommendedRefreshCodes(report FreshnessReport) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(report.StaleSecurities)+len(report.MissingNAVSecurities))
	appendCode := func(raw string) {
		code := NormalizeSecurityCode(raw)
		if code == "" {
			return
		}
		if _, ok := seen[code]; ok {
			return
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	for _, item := range report.StaleSecurities {
		appendCode(item.Code)
	}
	for _, item := range report.MissingNAVSecurities {
		appendCode(item.Code)
	}
	return out
}

// BatchPolicy carries the only per-protocol knobs a code batch legitimately
// needs. Everything else (order, ctx checks, soft-fail, status) is fixed so the
// three call sites cannot diverge.
type BatchPolicy struct {
	// Backoff is the ctx-aware pause between successful codes. Zero means no
	// pause: the scheduled job throttles upstream, while a synchronous REST
	// request or MCP tool call must not sleep inside its own deadline.
	Backoff time.Duration

	// FailureLogMessage names the per-code failure log so each protocol keeps
	// its own identifiable ops signal. Empty falls back to a generic message.
	FailureLogMessage string

	// LogAttrs are extra slog attributes for per-code failure logs (for example
	// a request id), letting an adapter keep its correlation context.
	LogAttrs []any
}

// BatchOutcome is the result of one code batch.
type BatchOutcome struct {
	// Done lists the codes that refreshed successfully, in input order.
	// Always non-nil so callers can serialize it without a nil guard.
	Done []string

	// Failed lists the codes whose refresh errored. It stays nil when nothing
	// failed, which is the shape the existing MCP and REST payloads serialize.
	Failed []string

	// Added is the total number of rows written across successful codes.
	Added int

	// Attempted counts non-blank codes that were actually dispatched.
	Attempted int

	// Stopped reports that ctx ended the batch before the list was exhausted.
	// Protocol adapters overlay their own meaning on it: MCP turns a
	// cancelled-with-no-work batch into a tool error, REST turns a hit request
	// deadline into 504. The batch itself never guesses the protocol.
	Stopped bool
}

// Status maps the outcome onto the shared crawl status vocabulary.
func (o BatchOutcome) Status() string {
	return BatchStatus(len(o.Done), o.Failed)
}

// BatchStatus maps ok/failed counts to complete | partial | error: complete
// when nothing failed, error when every attempted code failed, partial
// otherwise. One rule for REST, MCP and the job layer.
func BatchStatus(ok int, failed []string) string {
	if len(failed) == 0 {
		return "complete"
	}
	if ok == 0 {
		return "error"
	}
	return "partial"
}

// RunCodeBatch refreshes an explicit list of security codes, preserving input
// order. It is the one batch implementation behind the MCP crawl_nav tool, the
// admin crawl-nav endpoint and the scheduled price-refresh job.
//
// Semantics, identical for every caller:
//   - ctx is checked before each code; when ctx has ended the batch stops early
//     and reports Stopped rather than returning a protocol-specific error.
//   - blank codes are skipped and are not counted as attempted.
//   - a per-code failure is logged and soft-skipped into Failed, so one bad
//     upstream cannot abort the whole run; nothing is swallowed silently.
//   - Backoff pauses between successful codes only (a failed code is not
//     throttled), never after the final code, and the pause is ctx-aware.
func RunCodeBatch(ctx context.Context, codes []string, refresh CodeRefresher, policy BatchPolicy) BatchOutcome {
	outcome := BatchOutcome{Done: make([]string, 0, len(codes))}
	if refresh == nil {
		// Fail closed and loudly: a batch with no refresher would otherwise
		// look like a successful no-op crawl.
		slog.Error("crawl code batch has no refresher", "codes", len(codes))
		return outcome
	}
	for i, raw := range codes {
		if err := ctx.Err(); err != nil {
			outcome.Stopped = true
			return outcome
		}
		code := strings.TrimSpace(raw)
		if code == "" {
			continue
		}
		outcome.Attempted++
		added, err := refresh(ctx, code)
		if err != nil {
			logCodeFailure(policy, code, err)
			outcome.Failed = append(outcome.Failed, code)
			continue
		}
		outcome.Added += added
		outcome.Done = append(outcome.Done, code)
		if policy.Backoff > 0 && i < len(codes)-1 {
			if err := sleepBatch(ctx, policy.Backoff); err != nil {
				outcome.Stopped = true
				return outcome
			}
		}
	}
	return outcome
}

// StaleRefreshResult is one stale-only crawl run: which codes freshness
// recommended, and what the batch did with them.
type StaleRefreshResult struct {
	// Codes is the recommended refresh list. Empty means nothing was stale or
	// missing, so no upstream call was made at all.
	Codes []string

	// Batch is the per-code outcome. Done is non-nil even when Codes is empty.
	Batch BatchOutcome
}

// RefreshStaleCodes is the whole stale-only flow in one place: freshness report
// → recommended codes → batch refresh. All three protocols call this instead of
// each re-deriving the list and re-implementing the loop.
//
// The returned error is always the freshness read failure; per-code refresh
// failures never surface as an error because the batch soft-fails them into
// Batch.Failed. Callers own their own protocol response: status vocabulary via
// Batch.Status(), plus any cancellation or deadline overlay.
func RefreshStaleCodes(ctx context.Context, svc Service, refresh CodeRefresher, policy BatchPolicy) (StaleRefreshResult, error) {
	report, err := svc.GetFreshness(ctx)
	if err != nil {
		return StaleRefreshResult{}, err
	}
	codes := RecommendedRefreshCodes(report)
	if len(codes) == 0 {
		return StaleRefreshResult{Codes: codes, Batch: BatchOutcome{Done: make([]string, 0)}}, nil
	}
	return StaleRefreshResult{Codes: codes, Batch: RunCodeBatch(ctx, codes, refresh, policy)}, nil
}

// logCodeFailure records one per-code refresh failure with the caller's own
// message and correlation attributes. Every caught error is logged. Caller
// attributes lead so a protocol adapter keeps its correlation key (a request
// id) in the position its existing ops tooling already parses.
func logCodeFailure(policy BatchPolicy, code string, err error) {
	message := strings.TrimSpace(policy.FailureLogMessage)
	if message == "" {
		message = defaultBatchFailureLogMessage
	}
	attrs := make([]any, 0, len(policy.LogAttrs)+4)
	attrs = append(attrs, policy.LogAttrs...)
	attrs = append(attrs, "code", code, "error", err.Error())
	slog.Error(message, attrs...)
}

// sleepBatch waits d, or returns early with the ctx error when ctx ends, so a
// throttled batch never outlives its caller's deadline.
func sleepBatch(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
