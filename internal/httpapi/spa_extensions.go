package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

// registerSPAReadExtensions — session 门内的补充读端点（与 MCP 工具同一 service 层）。
// 挂载在 SessionAuth 组（读路径）。
func registerSPAReadExtensions(r chi.Router, portfolio *portfoliosvc.Service, admin adminsvc.Service) {
	r.Get("/api/transactions", handleListTransactions(portfolio))
	r.Get("/api/dca/plans", handleListDCAPlans(portfolio))
	r.Get("/api/alerts", handleCheckAlerts(admin))
	r.Get("/api/freshness", handleFreshness(admin))
}

// registerSPAWriteExtensions — 浏览器写扩展（session 或 EdgeKey 兼容），挂载在
// BrowserWriteAuth 组。MCP-only 能力的 REST 补面（docs/design/05 W4/W6）。
func registerSPAWriteExtensions(r chi.Router, portfolio *portfoliosvc.Service) {
	r.Post("/api/dca/plans", handleUpsertDCAPlan(portfolio))
	r.Post("/api/dca/plans/{id}/disable", handleDisableDCAPlan(portfolio))
	r.Post("/api/dca/run", handleRunDCA(portfolio))
	r.Post("/api/securities", handleUpsertSecurity(portfolio))
	r.Delete("/api/securities/{code}", handleDeleteSecurity(portfolio))
	r.Post("/api/portfolio/adjust-position", handleAdjustPosition(portfolio))
	r.Post("/api/reports", handleGenerateReport(portfolio))
}

// ── transactions 台账读 ─────────────────────────────────────────────

func handleListTransactions(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, ok := intQueryOpt(w, r, "limit", 200)
		if !ok {
			return
		}
		offset, ok := intQueryOpt(w, r, "offset", 0)
		if !ok {
			return
		}
		result, err := service.ListTransactions(r.Context(), portfoliosvc.ListTransactionsOptions{
			PortfolioID: portfolioIDFromRequest(r),
			FundCode:    q.Get("fund_code"),
			Direction:   q.Get("direction"),
			Search:      q.Get("search"),
			Limit:       limit,
			Offset:      offset,
			SortBy:      q.Get("sort"),
			SortDesc:    q.Get("sort_dir") == "desc",
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, result)
	}
}

// ── DCA 计划 ────────────────────────────────────────────────────────

func handleListDCAPlans(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		activeOnly := r.URL.Query().Get("active") == "true"
		plans, err := service.ListDCAPlans(r.Context(), portfoliosvc.ListDCAPlansOptions{
			ActiveOnly:  activeOnly,
			PortfolioID: portfolioIDFromRequest(r),
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"plans": plans})
	}
}

type upsertDCAPlanRequest struct {
	ID          int     `json:"id"`
	FundCode    string  `json:"fund_code"`
	FundName    string  `json:"fund_name"`
	Amount      float64 `json:"amount"`
	Frequency   string  `json:"frequency"`
	WeekdayMask string  `json:"weekday_mask"`
	TradeType   string  `json:"trade_type"`
	PortfolioID int     `json:"portfolio_id"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Active      *int    `json:"active"`
	Source      string  `json:"source"`
}

func handleUpsertDCAPlan(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var body upsertDCAPlanRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if body.FundCode == "" || body.Amount <= 0 {
			writeError(w, http.StatusBadRequest, "fund_code and positive amount required")
			return
		}
		source := body.Source
		if source == "" {
			// service 层默认 "mcp"（历史唯一调用方）；浏览器创建应如实归因 web。
			source = "web"
		}
		result, err := service.UpsertDCAPlan(r.Context(), portfoliosvc.UpsertDCAPlanInput{
			ID:          body.ID,
			FundCode:    body.FundCode,
			FundName:    body.FundName,
			Amount:      body.Amount,
			Frequency:   body.Frequency,
			WeekdayMask: body.WeekdayMask,
			TradeType:   body.TradeType,
			PortfolioID: body.PortfolioID,
			StartDate:   body.StartDate,
			EndDate:     body.EndDate,
			Active:      body.Active,
			Source:      source,
		})
		writeServiceResult(w, r, result, err)
	}
}

func handleDisableDCAPlan(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "id required")
			return
		}
		result, err := service.DisableDCAPlan(r.Context(), id)
		writeServiceResult(w, r, result, err)
	}
}

type runDCARequest struct {
	AsOf        string  `json:"as_of"`
	PortfolioID int     `json:"portfolio_id"`
	PlanID      int     `json:"plan_id"`
	FundCode    string  `json:"fund_code"`
	DryRun      bool    `json:"dry_run"`
	Mode        string  `json:"mode"`
	BaseAmount  float64 `json:"base_amount"`
}

func handleRunDCA(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var body runDCARequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		result, err := service.RunDCAAutoInvest(r.Context(), portfoliosvc.RunDCAAutoInvestInput{
			AsOf:        body.AsOf,
			PortfolioID: body.PortfolioID,
			PlanID:      body.PlanID,
			FundCode:    body.FundCode,
			DryRun:      body.DryRun,
			Mode:        body.Mode,
			BaseAmount:  body.BaseAmount,
		})
		writeServiceResult(w, r, result, err)
	}
}

// ── 证券主数据写 ────────────────────────────────────────────────────

type upsertSecurityRequest struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	FundType     string `json:"fund_type"`
	SecurityType string `json:"security_type"`
	Market       string `json:"market"`
	Currency     string `json:"currency"`
	Exchange     string `json:"exchange"`
	Source       string `json:"source"`
}

func handleUpsertSecurity(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var body upsertSecurityRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if body.Code == "" || body.Name == "" {
			writeError(w, http.StatusBadRequest, "code and name required")
			return
		}
		result, err := service.UpsertSecurity(r.Context(), portfoliosvc.UpsertSecurityInput{
			Code:         body.Code,
			Name:         body.Name,
			FundType:     body.FundType,
			SecurityType: body.SecurityType,
			Market:       body.Market,
			Currency:     body.Currency,
			Exchange:     body.Exchange,
			Source:       body.Source,
		})
		writeServiceResult(w, r, result, err)
	}
}

func handleDeleteSecurity(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := chi.URLParam(r, "code")
		if code == "" {
			writeError(w, http.StatusBadRequest, "code required")
			return
		}
		result, err := service.DeleteSecurity(r.Context(), code)
		writeServiceResult(w, r, result, err)
	}
}

// ── 持仓调整 / 报告 / 告警 ──────────────────────────────────────────

type adjustPositionRequest struct {
	Code        string  `json:"code"`
	Shares      float64 `json:"shares"`
	PortfolioID int     `json:"portfolio_id"`
	Reason      string  `json:"reason"`
}

func handleAdjustPosition(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var body adjustPositionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if body.Code == "" || body.Shares < 0 {
			writeError(w, http.StatusBadRequest, "code and non-negative shares required")
			return
		}
		result, err := service.AdjustPosition(r.Context(), portfoliosvc.AdjustPositionInput{
			Code:        body.Code,
			Shares:      body.Shares,
			PortfolioID: body.PortfolioID,
			Reason:      body.Reason,
		})
		writeServiceResult(w, r, result, err)
	}
}

type generateReportRequest struct {
	PortfolioID  int    `json:"portfolio_id"`
	Title        string `json:"title"`
	BaseCurrency string `json:"base_currency"`
	AsOf         string `json:"as_of"`
}

func handleGenerateReport(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var body generateReportRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if body.PortfolioID <= 0 {
			body.PortfolioID = portfolioIDFromRequest(r)
		}
		result, err := service.GenerateReport(r.Context(), portfoliosvc.GenerateReportInput{
			PortfolioID:  body.PortfolioID,
			Title:        body.Title,
			BaseCurrency: body.BaseCurrency,
			AsOf:         body.AsOf,
		})
		writeServiceResult(w, r, result, err)
	}
}

func handleCheckAlerts(service adminsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		priceChange, ok := floatQueryOpt(w, r, "price_change_pct", 0)
		if !ok {
			return
		}
		drawdown, ok := floatQueryOpt(w, r, "drawdown_pct", 0)
		if !ok {
			return
		}
		staleDays, ok := intQueryOpt(w, r, "stale_days", 0)
		if !ok {
			return
		}
		result, err := service.CheckAlerts(r.Context(), adminsvc.CheckAlertsInput{
			PriceChangePct: priceChange,
			DrawdownPct:    drawdown,
			StaleDays:      staleDays,
			PortfolioID:    portfolioIDFromRequest(r),
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, result)
	}
}

// handleFreshness 把 admin freshness 报告以 session 门暴露给 SPA 顶栏新鲜度徽章
// （facts-only 读，无管理面泄露）。
func handleFreshness(service adminsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.GetFreshness(r.Context())
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, report)
	}
}

// writeServiceResult 统一 service 写结果的错误映射。
// portfolio service 层用裸 fmt.Errorf 做校验（无 sentinel），所以这里用 400 语义：
// 安全的短校验消息（"xxx is required" 等）透传给前端，含 SQL/内部细节的降级为 bad_request。
func writeServiceResult(w http.ResponseWriter, r *http.Request, result any, err error) {
	if err != nil {
		writeSafeError(w, r, http.StatusBadRequest, err)
		return
	}
	WriteJSON(w, http.StatusOK, result)
}
