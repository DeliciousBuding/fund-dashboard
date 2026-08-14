package datasource

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// maxFundHoldingsRows soft-caps holdings rows per fund report (#241).
const maxFundHoldingsRows = 500

// FundHolding is one disclosed top holding for a mutual fund.
type FundHolding struct {
	StockCode   string
	StockName   string
	WeightPct   float64
	Shares      float64 // 万股 as reported
	MarketValue float64 // 万元 as reported
	ReportDate  string  // YYYY-MM-DD
}

// EastmoneyHoldings fetches quarterly fund holdings (jjcc) from fundf10.eastmoney.com.
type EastmoneyHoldings struct {
	client  *http.Client
	baseURL string
}

func NewEastmoneyHoldings() *EastmoneyHoldings {
	return &EastmoneyHoldings{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://fundf10.eastmoney.com",
	}
}

// FetchHoldings scrapes the latest available top-N holdings for a fund code.
// year may be empty to let the endpoint choose; when empty we try current and previous year.
func (s *EastmoneyHoldings) FetchHoldings(ctx context.Context, code string, topline int) ([]FundHolding, error) {
	code = normalizeFundCode(code)
	if code == "" {
		return nil, fmt.Errorf("fund code is required")
	}
	if topline <= 0 {
		topline = 10
	}
	years := []string{"", strconv.Itoa(time.Now().Year()), strconv.Itoa(time.Now().Year() - 1)}
	var lastErr error
	for _, year := range years {
		holdings, err := s.fetchYear(ctx, code, year, topline)
		if err != nil {
			lastErr = err
			continue
		}
		if len(holdings) > 0 {
			return holdings, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("eastmoney holdings(%s): empty holdings table", code)
}

func (s *EastmoneyHoldings) fetchYear(ctx context.Context, code, year string, topline int) ([]FundHolding, error) {
	url := fmt.Sprintf("%s/FundArchivesDatas.aspx?type=jjcc&code=%s&topline=%d&year=%s&month=&rt=%f",
		s.baseURL, code, topline, year, rand.Float64())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://fundf10.eastmoney.com/")
	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eastmoney holdings GET: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eastmoney holdings HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return parseHoldingsApidata(string(body))
}

var (
	reApidataContent = regexp.MustCompile(`content:"([\s\S]*?)",\s*arryear`)
	reReportDate     = regexp.MustCompile(`px12'>(\d{4}-\d{2}-\d{2})</font>`)
	reTR             = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	reTD             = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	reTags           = regexp.MustCompile(`(?is)<[^>]+>`)
)

func parseHoldingsApidata(data string) ([]FundHolding, error) {
	m := reApidataContent.FindStringSubmatch(data)
	if m == nil {
		return nil, fmt.Errorf("apidata content not found")
	}
	content := m[1]
	content = strings.ReplaceAll(content, `\"`, `"`)
	content = strings.ReplaceAll(content, `\'`, `'`)
	content = strings.ReplaceAll(content, `\/`, `/`)

	reportDate := ""
	if rd := reReportDate.FindStringSubmatch(content); rd != nil {
		reportDate = rd[1]
	}

	rows := reTR.FindAllStringSubmatch(content, -1)
	out := make([]FundHolding, 0, 16)
	for _, row := range rows {
		cells := reTD.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 5 {
			continue
		}
		texts := make([]string, len(cells))
		for i, c := range cells {
			texts[i] = strings.TrimSpace(reTags.ReplaceAllString(c[1], ""))
		}
		// rank, code, name, news, weight%, shares, market_value
		stockCode := texts[1]
		if stockCode == "" || stockCode == "股票代码" {
			continue
		}
		// skip header-like rows
		if _, err := strconv.Atoi(texts[0]); err != nil {
			continue
		}
		weight, _ := strconv.ParseFloat(strings.TrimSuffix(strings.ReplaceAll(texts[4], ",", ""), "%"), 64)
		var shares, mv float64
		if len(texts) > 5 {
			shares, _ = strconv.ParseFloat(strings.ReplaceAll(texts[5], ",", ""), 64)
		}
		if len(texts) > 6 {
			mv, _ = strconv.ParseFloat(strings.ReplaceAll(texts[6], ",", ""), 64)
		}
		out = append(out, FundHolding{
			StockCode:   stockCode,
			StockName:   texts[2],
			WeightPct:   weight,
			Shares:      shares,
			MarketValue: mv,
			ReportDate:  reportDate,
		})
	}
	if len(out) > maxFundHoldingsRows {
		out = out[:maxFundHoldingsRows]
	}
	return out, nil
}
