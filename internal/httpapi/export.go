package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"
)

// registerExportRoutes mounts SPA export helpers under /api/export.
// Body is client-supplied facts (no DB write). Public at app layer; edge may inject EdgeKey.
func registerExportRoutes(r chi.Router) {
	r.Post("/api/export/transactions-xlsx", handleExportTransactionsXLSX())
}

type exportTransactionsRequest struct {
	FundName     string                 `json:"fundName"`
	Transactions []exportTransactionRow `json:"transactions"`
}

type exportTransactionRow struct {
	TradeTime      string   `json:"trade_time"`
	ConfirmDate    string   `json:"confirm_date"`
	Direction      string   `json:"direction"`
	Type           string   `json:"type"`
	Amount         float64  `json:"amount"`
	Shares         float64  `json:"shares"`
	Nav            *float64 `json:"nav"`
	InferredNav    *float64 `json:"inferred_nav"`
	Fee            float64  `json:"fee"`
	SettlementDays *int     `json:"settlement_days"`
	TradeDayType   string   `json:"trade_day_type"`
}

// exportLang is a tiny label pack for XLSX headers/direction (mirrors SPA fundDetail.csv.* / dir.*).
type exportLang struct {
	headers []string
	dirMap  map[string]string
}

func exportLangFromRequest(r *http.Request) exportLang {
	// Prefer first tag; default zh (product primary locale). en* → English.
	al := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept-Language")))
	// Take primary range before comma/weight.
	if i := strings.IndexByte(al, ','); i >= 0 {
		al = al[:i]
	}
	if i := strings.IndexByte(al, ';'); i >= 0 {
		al = al[:i]
	}
	al = strings.TrimSpace(al)
	if strings.HasPrefix(al, "en") {
		return exportLang{
			headers: []string{
				"Trade time", "Confirm date", "Type", "Amount", "Shares",
				"Deal NAV", "Inferred NAV", "Fee", "Settlement", "Trade day",
			},
			dirMap: map[string]string{
				"buy": "Buy", "sell": "Sell", "dividend": "Dividend",
				"convert_in": "Convert in", "convert_out": "Convert out", "forced_redeem": "Forced redeem",
			},
		}
	}
	return exportLang{
		headers: []string{
			"交易时间", "确认日期", "类型", "金额", "份额",
			"成交净值", "推算净值", "手续费", "结算", "交易日",
		},
		dirMap: map[string]string{
			"buy": "买入", "sell": "卖出", "dividend": "分红",
			"convert_in": "转入", "convert_out": "转出", "forced_redeem": "强赎",
		},
	}
}

func handleExportTransactionsXLSX() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Bound decode cost: 2 MiB body, max 5000 rows (#201).
		const maxBody = 2 << 20
		const maxRows = 5000
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		var req exportTransactionsRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "invalid_json: empty body")
				return
			}
			// MaxBytesReader wraps as *http.MaxBytesError on Go 1.19+
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if len(req.Transactions) == 0 {
			writeError(w, http.StatusBadRequest, "transactions required")
			return
		}
		if len(req.Transactions) > maxRows {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("transactions max %d rows", maxRows))
			return
		}

		lang := exportLangFromRequest(r)

		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		sheet := f.GetSheetName(0)
		if sheet == "" {
			sheet = "Sheet1"
		}
		if err := f.SetSheetName(sheet, "transactions"); err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		sheet = "transactions"

		for i, h := range lang.headers {
			cell, err := excelize.CoordinatesToCellName(i+1, 1)
			if err != nil {
				writeSafeError(w, r, http.StatusInternalServerError, err)
				return
			}
			if err := f.SetCellValue(sheet, cell, h); err != nil {
				writeSafeError(w, r, http.StatusInternalServerError, err)
				return
			}
		}

		for rowIdx, tx := range req.Transactions {
			rIdx := rowIdx + 2
			dir := lang.dirMap[tx.Direction]
			if dir == "" {
				if tx.Type != "" {
					dir = tx.Type
				} else {
					dir = tx.Direction
				}
			}
			dir = clampExportCell(dir, 64)
			tradeTime := clampExportCell(tx.TradeTime, 40)
			if len(tradeTime) > 16 {
				tradeTime = tradeTime[:16]
			}
			settlement := ""
			if tx.SettlementDays != nil {
				settlement = fmt.Sprintf("T+%d", *tx.SettlementDays)
			}
			nav := ""
			if tx.Nav != nil {
				nav = fmt.Sprintf("%.4f", *tx.Nav)
			}
			inferred := ""
			if tx.InferredNav != nil {
				inferred = fmt.Sprintf("%.6f", *tx.InferredNav)
			}
			fee := ""
			if tx.Fee > 0 {
				fee = fmt.Sprintf("%.2f", tx.Fee)
			}
			values := []any{
				tradeTime,
				clampExportCell(tx.ConfirmDate, 40),
				dir,
				fmt.Sprintf("%.2f", tx.Amount),
				fmt.Sprintf("%.2f", tx.Shares),
				nav,
				inferred,
				fee,
				settlement,
				clampExportCell(tx.TradeDayType, 32),
			}
			for col, v := range values {
				cell, err := excelize.CoordinatesToCellName(col+1, rIdx)
				if err != nil {
					writeSafeError(w, r, http.StatusInternalServerError, err)
					return
				}
				if err := f.SetCellValue(sheet, cell, v); err != nil {
					writeSafeError(w, r, http.StatusInternalServerError, err)
					return
				}
			}
		}

		buf, err := f.WriteToBuffer()
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}

		fundName := strings.TrimSpace(req.FundName)
		if fundName == "" {
			fundName = "transactions"
		}
		// Sanitize filename for Content-Disposition (#231): no controls, bound length.
		safe := sanitizeExportFilename(fundName)
		filename := fmt.Sprintf("%s_transactions_%s.xlsx", safe, time.Now().Format("20060102"))

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		// RFC 6266: ASCII fallback + RFC 5987 UTF-8 name so CJK fund names survive (#231).
		asciiName := fmt.Sprintf("transactions_%s.xlsx", time.Now().Format("20060102"))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, asciiName, url.PathEscape(filename)))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(buf.Bytes()); err != nil {
			slog.Warn("export transactions xlsx: write failed", "err", err)
		}
	}
}

// sanitizeExportFilename strips path/control chars and caps length for Content-Disposition (#231).
func sanitizeExportFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "transactions"
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f: // controls incl. CR/LF
			continue
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "transactions"
	}
	// Cap runes (not bytes) for header safety.
	runes := []rune(out)
	if len(runes) > 80 {
		out = string(runes[:80])
	}
	return out
}

// clampExportCell bounds client-supplied XLSX cell strings (#232).
func clampExportCell(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	// Prefer rune-safe cut for CJK fund labels.
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}
