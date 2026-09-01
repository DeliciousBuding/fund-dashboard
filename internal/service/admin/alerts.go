package admin

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

// chinaLoc is the fund NAV calendar location (CN A-share / fund industry
// convention), matching portfolio.summary's chinaMarketLoc. Calendar-day math
// must use it so 00:00–08:00 local does not undercount stale days.
var chinaLoc = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*3600)
}()

type CheckAlertsInput struct {
	PriceChangePct float64 // absolute threshold, default 5
	DrawdownPct    float64 // absolute threshold, default 10
	StaleDays      int     // default 4
	PortfolioID    int
}

type AlertItem struct {
	Kind         string   `json:"kind"`
	Code         string   `json:"code"`
	Name         string   `json:"name,omitempty"`
	Severity     string   `json:"severity"`
	Message      string   `json:"message"`
	Value        *float64 `json:"value,omitempty"`
	Threshold    *float64 `json:"threshold,omitempty"`
	AsOf         string   `json:"as_of,omitempty"`
	SecurityType string   `json:"security_type,omitempty"`
	Market       string   `json:"market,omitempty"`
}

type CheckAlertsResult struct {
	OK               bool        `json:"ok"`
	Count            int         `json:"count"`
	Alerts           []AlertItem `json:"alerts"`
	CheckedAt        string      `json:"checked_at"`
	PriceChangePct   float64     `json:"price_change_pct"`
	DrawdownPct      float64     `json:"drawdown_pct"`
	StaleDays        int         `json:"stale_days"`
	PortfolioID      int         `json:"portfolio_id"`
	DecisionBoundary string      `json:"decision_boundary"`
	SideEffects      string      `json:"side_effects"`
	WebhookSent      bool        `json:"webhook_sent"`
}

// CheckAlerts scans held positions for price moves, drawdowns, stale NAV, and DCA days.
// Facts-only: no external webhook delivery.
func (s Service) CheckAlerts(ctx context.Context, in CheckAlertsInput) (CheckAlertsResult, error) {
	priceThr := in.PriceChangePct
	if priceThr <= 0 {
		priceThr = 5
	}
	ddThr := in.DrawdownPct
	if ddThr <= 0 {
		ddThr = 10
	}
	staleDays := in.StaleDays
	if staleDays <= 0 {
		staleDays = 4
	}
	portfolioID := in.PortfolioID
	if portfolioID <= 0 {
		portfolioID = 1
	}
	if portfolioID > 1000 {
		portfolioID = 1000
	}
	now := time.Now().In(chinaLoc)
	checkedAt := now.UTC().Format(time.RFC3339)

	alerts := []AlertItem{}

	// market lives on fund_details in production PG (portfolio_snapshot has no market column).
	rows, err := s.db.QueryContext(ctx, `
		SELECT ps.fund_code, COALESCE(ps.fund_name,''), COALESCE(ps.security_type,'fund'), COALESCE(fd.market,''),
			COALESCE(ps.held_shares,0), ps.latest_nav, ps.pnl_pct
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		WHERE COALESCE(ps.portfolio_id,1) = ? AND COALESCE(ps.held_shares,0) > 0.001
		ORDER BY ps.fund_code
		LIMIT 5000
	`, portfolioID)
	if err != nil {
		return CheckAlertsResult{}, fmt.Errorf("list held: %w", err)
	}
	defer rows.Close()

	type held struct {
		code, name, secType, market string
		shares                      float64
		latestNAV                   sql.NullFloat64
		pnlPct                      sql.NullFloat64
	}
	var helds []held
	for rows.Next() {
		var h held
		if err := rows.Scan(&h.code, &h.name, &h.secType, &h.market, &h.shares, &h.latestNAV, &h.pnlPct); err != nil {
			return CheckAlertsResult{}, err
		}
		h.code = clampAdminText(h.code, 32)
		h.name = clampAdminText(h.name, 200)
		h.secType = clampAdminText(h.secType, 32)
		h.market = clampAdminText(h.market, 32)
		helds = append(helds, h)
	}
	if err := rows.Err(); err != nil {
		return CheckAlertsResult{}, err
	}

	for _, h := range helds {
		var change sql.NullFloat64
		var navDate sql.NullString
		if err := s.db.QueryRowContext(ctx, `
			SELECT daily_change_pct, date FROM nav_history
			WHERE fund_code = ? ORDER BY date DESC LIMIT 1
		`, h.code).Scan(&change, &navDate); err != nil && err != sql.ErrNoRows {
			return CheckAlertsResult{}, fmt.Errorf("lookup nav change %s: %w", h.code, err)
		}
		if change.Valid && math.Abs(change.Float64) >= priceThr {
			v := change.Float64
			thr := priceThr
			sev := "medium"
			if math.Abs(v) >= priceThr*2 {
				sev = "high"
			}
			asOf := ""
			if navDate.Valid {
				asOf = navDate.String
			}
			alerts = append(alerts, AlertItem{
				Kind: "price_change", Code: h.code, Name: h.name, Severity: sev,
				Message:      clampAdminText(fmt.Sprintf("%s daily change %.2f%% exceeds ±%.2f%%", h.code, v, priceThr), 500),
				Value:        &v,
				Threshold:    &thr,
				AsOf:         clampAdminText(asOf, 40),
				SecurityType: h.secType,
				Market:       h.market,
			})
		}

		dd, peakDate, troughDate, err := s.maxDrawdownPct(ctx, h.code)
		if err != nil {
			return CheckAlertsResult{}, err
		}
		if dd >= ddThr {
			v := dd
			thr := ddThr
			sev := "medium"
			if dd >= ddThr*1.5 {
				sev = "high"
			}
			alerts = append(alerts, AlertItem{
				Kind: "drawdown", Code: h.code, Name: h.name, Severity: sev,
				Message:      clampAdminText(fmt.Sprintf("%s max drawdown %.2f%% exceeds %.2f%% (peak %s trough %s)", h.code, dd, ddThr, peakDate, troughDate), 500),
				Value:        &v,
				Threshold:    &thr,
				AsOf:         clampAdminText(troughDate, 40),
				SecurityType: h.secType,
				Market:       h.market,
			})
		}

		if navDate.Valid {
			dateStr := navDate.String
			if len(dateStr) > 10 {
				dateStr = dateStr[:10]
			}
			d, err := time.ParseInLocation("2006-01-02", dateStr, chinaLoc)
			if err == nil {
				today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, chinaLoc)
				days := int(today.Sub(d).Hours() / 24)
				if days >= staleDays {
					v := float64(days)
					thr := float64(staleDays)
					alerts = append(alerts, AlertItem{
						Kind: "stale_nav", Code: h.code, Name: h.name, Severity: "low",
						Message:      clampAdminText(fmt.Sprintf("%s NAV stale %d days (threshold %d)", h.code, days, staleDays), 500),
						Value:        &v,
						Threshold:    &thr,
						AsOf:         clampAdminText(navDate.String, 40),
						SecurityType: h.secType,
						Market:       h.market,
					})
				}
			}
		}
	}

	// weekday_mask uses 1=Mon ... 7=Sun
	goW := int(now.Weekday()) // 0=Sun
	maskDay := goW
	if goW == 0 {
		maskDay = 7
	}
	dcaRows, err := s.db.QueryContext(ctx, `
		SELECT id, fund_code, COALESCE(fund_name,''), amount, weekday_mask, portfolio_id
		FROM dca_plans
		WHERE active = 1 AND portfolio_id = ?
		LIMIT 5000
	`, portfolioID)
	if err != nil {
		msg := strings.ToLower(err.Error())
		// Missing dca_plans on older fixtures — skip dca_day alerts only.
		if !(strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined_table")) {
			return CheckAlertsResult{}, fmt.Errorf("list dca plans: %w", err)
		}
	} else {
		defer dcaRows.Close()
		for dcaRows.Next() {
			var id int
			var code, name, mask string
			var amount float64
			var pid int
			if err := dcaRows.Scan(&id, &code, &name, &amount, &mask, &pid); err != nil {
				return CheckAlertsResult{}, fmt.Errorf("scan dca plan: %w", err)
			}
			code = clampAdminText(code, 32)
			name = clampAdminText(name, 200)
			if weekdayMaskHit(mask, maskDay) {
				v := amount
				alerts = append(alerts, AlertItem{
					Kind: "dca_day", Code: code, Name: name, Severity: "info",
					Message: clampAdminText(fmt.Sprintf("DCA plan #%d for %s is scheduled today (amount %.2f)", id, code, amount), 500),
					Value:   &v,
					AsOf:    now.Format("2006-01-02"),
				})
			}
		}
		if err := dcaRows.Err(); err != nil {
			return CheckAlertsResult{}, fmt.Errorf("iterate dca plans: %w", err)
		}
	}

	return CheckAlertsResult{
		OK: true, Count: len(alerts), Alerts: alerts, CheckedAt: checkedAt,
		PriceChangePct: priceThr, DrawdownPct: ddThr, StaleDays: staleDays, PortfolioID: portfolioID,
		DecisionBoundary: "facts_only", SideEffects: "none", WebhookSent: false,
	}, nil
}

// maxDrawdownNavPoints caps alert drawdown scan to the most recent N points
// (chronological). Earlier ORDER BY date ASC LIMIT 120 used the earliest
// window and missed modern drawdowns on long histories (#221).
const maxDrawdownNavPoints = 2000

func (s Service) maxDrawdownPct(ctx context.Context, code string) (dd float64, peakDate, troughDate string, err error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT date, unit_nav FROM (
			SELECT date, unit_nav FROM nav_history
			WHERE fund_code = ? AND unit_nav IS NOT NULL AND unit_nav > 0
			ORDER BY date DESC
			LIMIT ?
		) t
		ORDER BY date ASC
	`, code, maxDrawdownNavPoints)
	if err != nil {
		return 0, "", "", fmt.Errorf("drawdown nav_history %s: %w", code, err)
	}
	defer rows.Close()
	peak := -1.0
	maxDD := 0.0
	var peakD, troughD string
	curPeakD := ""
	for rows.Next() {
		var d string
		var nav float64
		if err := rows.Scan(&d, &nav); err != nil {
			return 0, "", "", fmt.Errorf("drawdown scan %s: %w", code, err)
		}
		if peak < 0 || nav > peak {
			peak = nav
			curPeakD = d
		}
		if peak > 0 {
			drop := (peak - nav) / peak * 100
			if drop > maxDD {
				maxDD = drop
				peakD = curPeakD
				troughD = d
			}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, "", "", fmt.Errorf("drawdown iterate %s: %w", code, err)
	}
	return maxDD, peakD, troughD, nil
}

func weekdayMaskHit(mask string, day int) bool {
	mask = strings.TrimSpace(mask)
	if mask == "" {
		return false
	}
	parts := strings.Split(mask, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "-") {
			var a, b int
			if _, err := fmt.Sscanf(p, "%d-%d", &a, &b); err == nil {
				if day >= a && day <= b {
					return true
				}
			}
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil && n == day {
			return true
		}
	}
	return false
}
