package datasource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// holdingsSampleBody is a cropped sample of the fundf10 jjcc apidata format
// (public well-known tickers only; no accounts or secrets).
const holdingsSampleBody = `var apidata={ content:"<div class='box'><h4><font class='px12'>2025-12-31</font></h4><table><tbody>` +
	`<tr><td>1</td><td class='toc'><a href='//quote.eastmoney.com/unify/r/105.NVDA' >NVDA</a></td>` +
	`<td class='toc'><a>英伟达</a></td><td>--</td><td>8.51%</td><td>25.08</td><td>32875.08</td></tr>` +
	`<tr><td>2</td><td><a>AAPL</a></td><td>苹果</td><td>--</td><td>7.54%</td><td>15.25</td><td>29140.26</td></tr>` +
	`</tbody></table></div>", arryear:[2025,2024], curyear:2025};`

func newEastmoneyHoldingsForTest(server *httptest.Server) *EastmoneyHoldings {
	return &EastmoneyHoldings{
		client:  server.Client(),
		baseURL: server.URL,
	}
}

func TestParseHoldingsApidata(t *testing.T) {
	sample := `var apidata={ content:"<div class='box'><h4><font class='px12'>2025-12-31</font></h4><table><tbody>` +
		`<tr><td>1</td><td class='toc'><a href='//quote.eastmoney.com/unify/r/105.NVDA' >NVDA</a></td>` +
		`<td class='toc'><a>英伟达</a></td><td>--</td><td>8.51%</td><td>25.08</td><td>32875.08</td></tr>` +
		`<tr><td>2</td><td><a>AAPL</a></td><td>苹果</td><td>--</td><td>7.54%</td><td>15.25</td><td>29140.26</td></tr>` +
		`</tbody></table></div>", arryear:[2025,2024], curyear:2025};`
	holdings, err := parseHoldingsApidata(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(holdings) != 2 {
		t.Fatalf("len=%d want 2", len(holdings))
	}
	if holdings[0].StockCode != "NVDA" || holdings[0].WeightPct != 8.51 {
		t.Fatalf("first=%+v", holdings[0])
	}
	if holdings[0].ReportDate != "2025-12-31" {
		t.Fatalf("report_date=%q", holdings[0].ReportDate)
	}
	if holdings[1].StockCode != "AAPL" || holdings[1].StockName != "苹果" {
		t.Fatalf("second=%+v", holdings[1])
	}
}

func TestParseHoldingsApidataRejectsDirtyWeight(t *testing.T) {
	sample := `var apidata={ content:"<div class='box'><h4><font class='px12'>2025-12-31</font></h4><table><tbody>` +
		`<tr><td>1</td><td class='toc'><a href='//quote.eastmoney.com/unify/r/105.NVDA' >NVDA</a></td>` +
		`<td class='toc'><a>英伟达</a></td><td>--</td><td>--</td><td>25.08</td><td>32875.08</td></tr>` +
		`</tbody></table></div>", arryear:[2025,2024], curyear:2025};`
	if _, err := parseHoldingsApidata(sample); err == nil {
		t.Fatal("expected error for dirty weight cell, got nil")
	}
}

func TestParseHoldingsApidataMissingReportDate(t *testing.T) {
	sample := `var apidata={ content:"<div class='box'><table><tbody>` +
		`<tr><td>1</td><td><a>NVDA</a></td><td>英伟达</td><td>--</td><td>8.51%</td><td>25.08</td><td>32875.08</td></tr>` +
		`</tbody></table></div>", arryear:[2025], curyear:2025};`
	holdings, err := parseHoldingsApidata(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(holdings) != 1 || holdings[0].ReportDate != "" {
		t.Fatalf("holdings=%+v, want one row with empty report date", holdings)
	}
}

func TestParseHoldingsApidataMissingSharesAndMarketValue(t *testing.T) {
	sample := `var apidata={ content:"<div class='box'><h4><font class='px12'>2025-12-31</font></h4><table><tbody>` +
		`<tr><td>1</td><td><a>NVDA</a></td><td>英伟达</td><td>--</td><td>8.51%</td></tr>` +
		`</tbody></table></div>", arryear:[2025], curyear:2025};`
	holdings, err := parseHoldingsApidata(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(holdings) != 1 {
		t.Fatalf("len=%d want 1", len(holdings))
	}
	if holdings[0].Shares != 0 || holdings[0].MarketValue != 0 {
		t.Fatalf("holding=%+v, want zero shares/market_value when cells are absent", holdings[0])
	}
}

func TestParseHoldingsApidataSkipsHeaderAndMalformedRankRows(t *testing.T) {
	sample := `var apidata={ content:"<div class='box'><h4><font class='px12'>2025-12-31</font></h4><table><tbody>` +
		`<tr><td>序号</td><td>股票代码</td><td>股票名称</td><td>相关资讯</td><td>占净值比例</td><td>持股数（万股）</td><td>持仓市值（万元）</td></tr>` +
		`<tr><td>--</td><td>BAD</td><td>坏行</td><td>--</td><td>1.00%</td><td>1</td><td>2</td></tr>` +
		`<tr><td>1</td><td>NVDA</td><td>英伟达</td><td>--</td><td>8.51%</td><td>25.08</td><td>32875.08</td></tr>` +
		`</tbody></table></div>", arryear:[2025], curyear:2025};`
	holdings, err := parseHoldingsApidata(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(holdings) != 1 || holdings[0].StockCode != "NVDA" {
		t.Fatalf("holdings=%+v, want only the valid NVDA row", holdings)
	}
}

func TestParseHoldingsApidataMalformedContent(t *testing.T) {
	if _, err := parseHoldingsApidata(`var apidata={ arryear:[2025], curyear:2025};`); err == nil {
		t.Fatal("expected error when apidata content is missing")
	}
}

func TestFetchHoldings_HTTP(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(holdingsSampleBody))
	}))
	defer server.Close()

	orig := newEastmoneyHoldingsForTest(server)
	holdings, err := orig.FetchHoldings(context.Background(), "110022", 10)
	if err != nil {
		t.Fatalf("FetchHoldings: %v", err)
	}
	if len(holdings) != 2 {
		t.Fatalf("len=%d want 2", len(holdings))
	}
	if holdings[0].StockCode != "NVDA" || holdings[0].WeightPct != 8.51 ||
		holdings[0].ReportDate != "2025-12-31" {
		t.Fatalf("first=%+v", holdings[0])
	}
	if holdings[1].StockCode != "AAPL" || holdings[1].MarketValue != 29140.26 {
		t.Fatalf("second=%+v", holdings[1])
	}
	if !strings.Contains(gotQuery, "code=110022") || !strings.Contains(gotQuery, "topline=10") {
		t.Fatalf("query=%q missing code/tohline", gotQuery)
	}
}

func TestFetchHoldings_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	orig := newEastmoneyHoldingsForTest(server)
	if _, err := orig.FetchHoldings(context.Background(), "110022", 10); err == nil {
		t.Fatal("expected error for upstream HTTP 503")
	}
}

func TestFetchHoldings_EmptyTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var apidata={ content:"<div class='box'><table><tbody></tbody></table></div>", arryear:[2025], curyear:2025};`))
	}))
	defer server.Close()

	orig := newEastmoneyHoldingsForTest(server)
	if _, err := orig.FetchHoldings(context.Background(), "110022", 10); err == nil {
		t.Fatal("expected empty holdings table error")
	}
}

func TestFetchHoldings_TriesCurrentYearAfterEmptyDefault(t *testing.T) {
	var sawEmptyYear, sawCurrentYear bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("year") == "" {
			sawEmptyYear = true
			w.Write([]byte(`var apidata={ content:"<div class='box'><table><tbody></tbody></table></div>", arryear:[2025], curyear:2025};`))
			return
		}
		sawCurrentYear = true
		w.Write([]byte(holdingsSampleBody))
	}))
	defer server.Close()

	orig := newEastmoneyHoldingsForTest(server)
	holdings, err := orig.FetchHoldings(context.Background(), "110022", 10)
	if err != nil {
		t.Fatalf("FetchHoldings: %v", err)
	}
	if !sawEmptyYear || !sawCurrentYear {
		t.Fatalf("sawEmptyYear=%v sawCurrentYear=%v, want both attempts", sawEmptyYear, sawCurrentYear)
	}
	if len(holdings) != 2 {
		t.Fatalf("len=%d want 2", len(holdings))
	}
}

func TestFetchHoldings_RequiresFundCode(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer server.Close()

	orig := newEastmoneyHoldingsForTest(server)
	for _, code := range []string{"", "   "} {
		if _, err := orig.FetchHoldings(context.Background(), code, 10); err == nil {
			t.Fatalf("FetchHoldings(%q): expected error", code)
		}
	}
	if requests != 0 {
		t.Fatalf("upstream requests = %d, want 0 (empty code rejected before network)", requests)
	}
}
