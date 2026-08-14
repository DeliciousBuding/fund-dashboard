package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"time"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

func registerMarketRoutes(r chi.Router, service *portfoliosvc.Service) {
	r.Get("/api/market/exchange-rate", handleExchangeRate(service))
	r.Get("/api/market/indices", handleMarketIndices(service))
	r.Get("/api/market/stream", handleMarketStream(service))
	// Single-index live + history for SPA NasdaqOverview (#95).
	r.Get("/api/market/index/{code}", handleIndexLive(service))
	r.Get("/api/market/index/{code}/history", handleIndexHistory(service))
	r.Get("/api/stocks/{code}", handleUSStock(service))
}

func handleExchangeRate(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.GetExchangeRate(r.Context())
		if err != nil {
			writeSafeError(w, r, http.StatusBadGateway, err)
			return
		}
		WriteJSON(w, http.StatusOK, report)
	}
}

func handleMarketIndices(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.GetMarketIndices(r.Context())
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		out := make([]map[string]any, 0, len(report.Indices))
		for code, item := range report.Indices {
			out = append(out, map[string]any{
				"code":       code,
				"name":       item.Name,
				"market":     item.Market,
				"price":      item.Price,
				"change_pct": item.ChangePct,
				"change_amt": item.Change,
				"updated_at": item.UpdatedAt,
			})
		}
		// Map iteration order is non-deterministic; SPA/tests need stable order.
		sort.Slice(out, func(i, j int) bool {
			ci, _ := out[i]["code"].(string)
			cj, _ := out[j]["code"].(string)
			return ci < cj
		})
		WriteJSON(w, http.StatusOK, out)
	}
}

func handleIndexLive(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code, _ := url.PathUnescape(chi.URLParam(r, "code"))
		report, err := service.GetIndexLive(r.Context(), code)
		if err != nil {
			writeSafeError(w, r, http.StatusBadGateway, err)
			return
		}
		WriteJSON(w, http.StatusOK, report)
	}
}

func handleIndexHistory(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code, _ := url.PathUnescape(chi.URLParam(r, "code"))
		rangeKey := r.URL.Query().Get("range")
		interval := r.URL.Query().Get("interval")
		report, err := service.GetIndexHistory(r.Context(), code, rangeKey, interval)
		if err != nil {
			writeSafeError(w, r, http.StatusBadGateway, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"symbol":            report.Symbol,
			"count":             report.Count,
			"range":             report.Range,
			"data":              report.Data,
			"source":            report.Source,
			"external_fetch":    report.ExternalFetch,
			"decision_boundary": report.DecisionBoundary,
			"side_effects":      report.SideEffects,
		})
	}
}

func handleUSStock(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.GetUSStock(r.Context(), portfoliosvc.USStockOptions{
			Symbol:         chi.URLParam(r, "code"),
			Range:          r.URL.Query().Get("range"),
			IncludeHistory: boolQueryDefault(r, "include_history", true),
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		// SPA USStockInfoSchema is flat (#98); MCP/tools keep nested report via service.
		WriteJSON(w, http.StatusOK, usStockSPAResponse(report))
	}
}

// usStockSPAResponse maps nested service report to contracts USStockInfoSchema.
func usStockSPAResponse(report portfoliosvc.USStockReport) map[string]any {
	out := map[string]any{
		"code":              report.Symbol,
		"name":              report.Symbol,
		"market":            "us",
		"price":             0.0,
		"previous_close":    0.0,
		"change":            0.0,
		"change_pct":        0.0,
		"high":              0.0,
		"low":               0.0,
		"open":              0.0,
		"volume":            0.0,
		"currency":          "USD",
		"market_time":       "",
		"profile":           nil,
		"history":           []any{},
		"source":            report.ExternalFetch,
		"decision_boundary": report.DecisionBoundary,
		"side_effects":      report.SideEffects,
		"external_fetch":    report.ExternalFetch,
	}
	if report.Error != "" {
		out["error"] = report.Error
		out["message"] = report.Message
	}
	if report.Quote != nil {
		q := report.Quote
		out["name"] = q.Name
		if q.Name == "" {
			out["name"] = report.Symbol
		}
		out["price"] = q.Price
		out["previous_close"] = q.PreviousClose
		out["change"] = q.Change
		out["change_pct"] = q.ChangePct
		out["high"] = q.High
		out["low"] = q.Low
		out["open"] = q.Open
		out["volume"] = q.Volume
		if q.Currency != "" {
			out["currency"] = q.Currency
		}
		out["market_time"] = q.MarketTime
		if out["source"] == "" || out["source"] == "not_performed" {
			out["source"] = "cache"
		}
	}
	if report.Profile != nil {
		p := report.Profile
		out["profile"] = map[string]any{
			"sector":      p.Sector,
			"industry":    p.Industry,
			"market_cap":  p.MarketCap,
			"pe":          p.PE,
			"description": p.Description,
		}
	}
	if report.History != nil {
		hist := make([]map[string]any, 0, len(report.History.Data))
		for _, pt := range report.History.Data {
			hist = append(hist, map[string]any{
				"date":       pt.Date,
				"close":      pt.Close,
				"change_pct": pt.ChangePct,
			})
		}
		out["history"] = hist
	}
	if report.ExternalFetch == "yahoo_chart" {
		out["source"] = "yahoo_chart"
	}
	return out
}

func boolQueryDefault(r *http.Request, key string, fallback bool) bool {
	switch r.URL.Query().Get(key) {
	case "":
		return fallback
	case "1", "true", "TRUE", "True":
		return true
	default:
		return false
	}
}

// marketStreamMaxLifetime caps a single SSE connection so sockets do not live forever.
// SPA useSSE reconnects with exponential backoff when the stream ends.
// Overridable in tests.
var marketStreamMaxLifetime = 20 * time.Minute

// clearSSEWriteDeadline removes the http.Server WriteTimeout deadline from this
// connection so long-lived SSE is not cut at 60s. Requires Go 1.20+ ResponseController;
// no-op (logged at debug) when the ResponseWriter does not support deadlines
// (e.g. httptest.ResponseRecorder).
func clearSSEWriteDeadline(w http.ResponseWriter) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		slog.Debug("market stream clear write deadline unsupported", "error", err.Error())
	}
}

func handleMarketStream(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Disable server WriteTimeout for this long-lived stream; still cap lifetime below.
		// Single-user product: no process-wide SSE semaphore (cap was noise at RPM≪1).
		clearSSEWriteDeadline(w)

		// Cap max SSE lifetime; clients should reconnect (useSSE already does).
		streamCtx, cancel := context.WithTimeout(r.Context(), marketStreamMaxLifetime)
		defer cancel()

		// Middleware may wrap ResponseWriter without http.Flusher; still stream bytes.
		flusher, _ := w.(http.Flusher)
		flush := func() {
			if flusher != nil {
				flusher.Flush()
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		// Hint clients that the server may close after max lifetime; reconnect is expected.
		w.Header().Set("X-SSE-Max-Lifetime-Seconds", fmt.Sprintf("%d", int(marketStreamMaxLifetime.Seconds())))
		w.WriteHeader(http.StatusOK)
		flush()

		writeIndices := func() error {
			report, err := service.GetMarketIndices(streamCtx)
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0, len(report.Indices))
			for code, item := range report.Indices {
				out = append(out, map[string]any{
					"code":       code,
					"name":       item.Name,
					"market":     item.Market,
					"price":      item.Price,
					"change_pct": item.ChangePct,
					"change_amt": item.Change,
					"updated_at": item.UpdatedAt,
				})
			}
			payload, err := json.Marshal(out)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "event: indices\ndata: %s\n\n", payload); err != nil {
				return err
			}
			flush()
			return nil
		}

		// Initial snapshot so SPA paints immediately.
		// SSE warn comments must not embed err (#240).
		if err := writeIndices(); err != nil {
			slog.Debug("market stream initial indices failed", "request_id", RequestIDFromContext(streamCtx), "error", err.Error())
			_, _ = fmt.Fprintf(w, ": warn upstream_unavailable\n\n")
			flush()
		}

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-streamCtx.Done():
				// Client disconnect, server shutdown, or max lifetime — SPA reconnects.
				return
			case <-heartbeat.C:
				if _, err := fmt.Fprintf(w, ": ping %d\n\n", time.Now().Unix()); err != nil {
					return
				}
				flush()
			case <-ticker.C:
				if err := writeIndices(); err != nil {
					slog.Debug("market stream indices refresh failed", "request_id", RequestIDFromContext(streamCtx), "error", err.Error())
					_, _ = fmt.Fprintf(w, ": warn upstream_unavailable\n\n")
					flush()
				}
			}
		}
	}
}
