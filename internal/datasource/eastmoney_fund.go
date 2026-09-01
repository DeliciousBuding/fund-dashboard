package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Compile-time check: EastmoneyFund satisfies PriceSource.
var _ PriceSource = (*EastmoneyFund)(nil)

// EastmoneyFund fetches Chinese mutual fund NAV history from eastmoney.com.
// It uses the public pingzhongdata JS endpoint — no auth required.
type EastmoneyFund struct {
	client    *http.Client
	serverURL string // empty means use default eastmoney URLs
}

// NewEastmoneyFund creates an EastmoneyFund with a sensible default timeout.
func NewEastmoneyFund() *EastmoneyFund {
	return &EastmoneyFund{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *EastmoneyFund) baseURL() string {
	if s.serverURL != "" {
		return s.serverURL
	}
	return "https://fund.eastmoney.com"
}

var (
	eastmoneyNameRe = regexp.MustCompile(`var fS_name = "(.*?)";`)
	eastmoneyCodeRe = regexp.MustCompile(`var fS_code = "(.*?)";`)
	eastmoneyDateRe = regexp.MustCompile(`var fS_buyMinDate = "(.*?)";`)
)

func (s *EastmoneyFund) FetchHistory(ctx context.Context, code string) ([]PricePoint, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("fund code is required")
	}
	code = normalizeFundCode(code)
	data, err := s.get(ctx, fmt.Sprintf(
		"%s/pingzhongdata/%s.js", s.baseURL(), url.PathEscape(code),
	))
	if err != nil {
		return nil, fmt.Errorf("eastmoney FetchHistory(%s): %w", code, err)
	}

	// Standard funds — Data_netWorthTrend
	if points := parseNetWorthTrend(data); len(points) > 0 {
		return points, nil
	}

	// Money-market funds — Data_millionCopiesIncome (unit NAV ≈ 1.0)
	if points := parseMillionCopiesIncome(data); len(points) > 0 {
		return points, nil
	}

	// HTTP succeeded but neither series parsed — treat as failure so refresh
	// does not look healthy with 0 rows.
	return nil, fmt.Errorf("eastmoney FetchHistory(%s): no NAV series parsed from response", code)
}

func (s *EastmoneyFund) FetchMeta(ctx context.Context, code string) (*FundMeta, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("fund code is required")
	}
	code = normalizeFundCode(code)
	data, err := s.get(ctx, fmt.Sprintf(
		"%s/pingzhongdata/%s.js", s.baseURL(), url.PathEscape(code),
	))
	if err != nil {
		return nil, fmt.Errorf("eastmoney FetchMeta(%s): %w", code, err)
	}

	nameMatch := eastmoneyNameRe.FindStringSubmatch(data)
	typeMatch := eastmoneyCodeRe.FindStringSubmatch(data)
	if nameMatch == nil || typeMatch == nil {
		return nil, nil
	}

	return &FundMeta{
		Code:      code,
		Name:      nameMatch[1],
		Type:      typeMatch[1],
		Inception: inception(data, eastmoneyDateRe),
	}, nil
}

// ── transport ───────────────────────────────────────────────────────────────

func (s *EastmoneyFund) get(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("eastmoney fund HTTP %d", resp.StatusCode)
	}

	body, err := readBodyLimited(resp.Body, 4<<20) // 4 MiB
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ── parsers ─────────────────────────────────────────────────────────────────

var (
	reNetWorth = regexp.MustCompile(`var Data_netWorthTrend = (\[.*?\]);`)
	reMillion  = regexp.MustCompile(`var Data_millionCopiesIncome = (\[.*?\]);`)
)

// maxNAVHistoryPoints soft-caps upstream NAV series before persist (#241).
const maxNAVHistoryPoints = 5000

// Eastmoney x fields are Unix milliseconds at Beijing midnight (UTC+8; China
// has had no DST since 1991). Formatting them in UTC shifts every NAV date one
// day back (2026-09-01 00:00 +08:00 == 2026-08-31T16:00Z), so date strings are
// produced in a fixed UTC+8 zone rather than a tzdata-dependent location.
// eastmoneyLocation is a fixed UTC+8 zone (China has had no DST since 1991),
// hoisted so per-point formatting does not allocate a new location.
var eastmoneyLocation = time.FixedZone("Asia/Shanghai", 8*3600)

func formatEastmoneyDate(ms int64) string {
	return time.UnixMilli(ms).In(eastmoneyLocation).Format("2006-01-02")
}

func capPricePoints(points []PricePoint) []PricePoint {
	if len(points) <= maxNAVHistoryPoints {
		return points
	}
	return points[len(points)-maxNAVHistoryPoints:]
}

func parseNetWorthTrend(data string) []PricePoint {
	match := reNetWorth.FindStringSubmatch(data)
	if match == nil {
		return nil
	}

	type rawPoint struct {
		X            float64  `json:"x"`
		Y            float64  `json:"y"`
		EquityReturn *float64 `json:"equityReturn"`
	}
	var raw []rawPoint
	if err := json.Unmarshal([]byte(match[1]), &raw); err != nil {
		return nil
	}

	points := make([]PricePoint, 0, len(raw))
	for _, r := range raw {
		if r.X <= 0 {
			continue // zero/missing timestamp must not surface as 1970-01-01
		}
		if r.Y < 0.01 || r.Y > 100 {
			continue // outlier
		}
		changePct := 0.0
		if r.EquityReturn != nil {
			changePct = *r.EquityReturn
		}
		points = append(points, PricePoint{
			Date:      formatEastmoneyDate(int64(r.X)),
			Price:     math.Round(r.Y*1e4) / 1e4,
			ChangePct: changePct,
		})
	}
	return capPricePoints(points)
}

func parseMillionCopiesIncome(data string) []PricePoint {
	match := reMillion.FindStringSubmatch(data)
	if match == nil {
		return nil
	}

	var raw [][]float64
	if err := json.Unmarshal([]byte(match[1]), &raw); err != nil {
		return nil
	}

	points := make([]PricePoint, 0, len(raw))
	for _, r := range raw {
		if len(r) < 2 || r[0] <= 0 {
			continue // malformed row or zero/missing timestamp
		}
		points = append(points, PricePoint{
			Date:      formatEastmoneyDate(int64(r[0])),
			Price:     1.0,
			ChangePct: r[1],
		})
	}
	return capPricePoints(points)
}

func inception(data string, re *regexp.Regexp) string {
	if m := re.FindStringSubmatch(data); m != nil {
		return m[1]
	}
	return ""
}

// normalizeFundCode pads all-numeric codes to 6 digits; uppercases tickers.
func normalizeFundCode(code string) string {
	s := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, code)
	if len(s) == len(code) {
		return fmt.Sprintf("%06s", s)
	}
	return strings.ToUpper(code)
}
