package datasource

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchHistory_StandardFund(t *testing.T) {
	// 2026-07-01T00:00:00Z = 1782864000000 ms
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var Data_netWorthTrend = [{"x":1782864000000,"y":1.2345,"equityReturn":0.52}];`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	ctx := context.Background()
	points, err := orig.FetchHistory(ctx, "019173")
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].Price != 1.2345 {
		t.Fatalf("expected price 1.2345, got %f", points[0].Price)
	}
	if points[0].ChangePct != 0.52 {
		t.Fatalf("expected change_pct 0.52, got %f", points[0].ChangePct)
	}
	if points[0].Date != "2026-07-01" {
		t.Fatalf("expected date 2026-07-01, got %s", points[0].Date)
	}
}

func TestFetchHistory_MoneyFund(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var Data_millionCopiesIncome = [[1782864000000,0.5432]];`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	ctx := context.Background()
	points, err := orig.FetchHistory(ctx, "000001")
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].Price != 1.0 {
		t.Fatalf("expected nav 1.0 for money fund, got %f", points[0].Price)
	}
	if points[0].ChangePct != 0.5432 {
		t.Fatalf("expected change_pct 0.5432, got %f", points[0].ChangePct)
	}
}

func TestFetchHistory_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var unrelated = [];`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	ctx := context.Background()
	points, err := orig.FetchHistory(ctx, "019173")
	if err == nil {
		t.Fatalf("FetchHistory: expected error for unparsed body, got points=%v", points)
	}
	if points != nil {
		t.Fatalf("expected nil points on parse failure, got %v", points)
	}
}

func TestFetchHistory_OutlierFiltered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var Data_netWorthTrend = [{"x":1751414400000,"y":0.001,"equityReturn":0},{"x":1751500800000,"y":150.0,"equityReturn":0}];`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	ctx := context.Background()
	points, err := orig.FetchHistory(ctx, "019173")
	// All points filtered as outliers → no usable series → error (not silent empty success).
	if err == nil {
		t.Fatalf("FetchHistory: expected error when only outlier points remain, got points=%v", points)
	}
}

func TestFetchMeta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var fS_name = "测试基金";var fS_code = "QDII";var fS_buyMinDate = "2020-01-15";`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	ctx := context.Background()
	meta, err := orig.FetchMeta(ctx, "019173")
	if err != nil {
		t.Fatalf("FetchMeta: %v", err)
	}
	if meta == nil {
		t.Fatal("expected meta, got nil")
	}
	if meta.Name != "测试基金" {
		t.Fatalf("expected name 测试基金, got %s", meta.Name)
	}
	if meta.Type != "QDII" {
		t.Fatalf("expected type QDII, got %s", meta.Type)
	}
	if meta.Inception != "2020-01-15" {
		t.Fatalf("expected inception 2020-01-15, got %s", meta.Inception)
	}
}

func TestFetchMeta_NoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var unrelated = 1;`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	meta, _ := orig.FetchMeta(context.Background(), "019173")
	if meta != nil {
		t.Fatal("expected nil meta for no match")
	}
}

func TestNormalizeFundCode(t *testing.T) {
	tests := []struct{ in, want string }{
		{"019173", "019173"},
		{"19173", "019173"},
		{"aapl", "AAPL"},
		{"AAPL", "AAPL"},
		{"600519", "600519"},
	}
	for _, tc := range tests {
		got := normalizeFundCode(tc.in)
		if got != tc.want {
			t.Errorf("normalizeFundCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestFetchHistory_UsesEastmoneyTimezone locks the UTC+8 date conversion: the
// upstream x timestamp is Beijing midnight, which is 16:00 UTC on the previous
// day. Formatting in UTC used to store every NAV one day early.
func TestFetchHistory_UsesEastmoneyTimezone(t *testing.T) {
	beijingMidnight := time.Date(2026, 7, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600)).UnixMilli()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `var Data_netWorthTrend = [{"x":%d,"y":1.2345,"equityReturn":0.1}];`, beijingMidnight)
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	points, err := orig.FetchHistory(context.Background(), "019173")
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}
	if len(points) != 1 || points[0].Date != "2026-07-01" {
		t.Fatalf("points=%+v, want date 2026-07-01 for Beijing-midnight timestamp", points)
	}
}

// TestParseNetWorthTrendSkipsZeroAndNegativeTimestamps guards against
// zero-value/missing x becoming 1970-01-01 points.
func TestParseNetWorthTrendSkipsZeroAndNegativeTimestamps(t *testing.T) {
	data := `var Data_netWorthTrend = [{"x":0,"y":1.1},{"x":-1,"y":1.2},{"x":1751414400000,"y":1.3}];`
	points := parseNetWorthTrend(data)
	if len(points) != 1 {
		t.Fatalf("len=%d want 1 (zero/negative timestamps dropped)", len(points))
	}
	if points[0].Date != "2025-07-02" {
		t.Fatalf("date=%q, want 2025-07-02 (UTC 00:00 renders as +08:00 same day)", points[0].Date)
	}
}

// TestParseMillionCopiesIncomeSkipsBadRows covers malformed/zero-timestamp rows.
func TestParseMillionCopiesIncomeSkipsBadRows(t *testing.T) {
	data := `var Data_millionCopiesIncome = [[0,0.1],[1751414400000,0.22],[1751500800000]];`
	points := parseMillionCopiesIncome(data)
	if len(points) != 1 {
		t.Fatalf("len=%d want 1 (zero timestamp and short row dropped)", len(points))
	}
	if points[0].ChangePct != 0.22 {
		t.Fatalf("change_pct=%f want 0.22", points[0].ChangePct)
	}
}

// TestFetchHistory_EmptyCodeRejectedBeforeNetwork ensures empty codes never
// become the synthesized 000000 fund and hit upstream.
func TestFetchHistory_EmptyCodeRejectedBeforeNetwork(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	for _, code := range []string{"", "   "} {
		if _, err := orig.FetchHistory(context.Background(), code); err == nil {
			t.Fatalf("FetchHistory(%q): expected error", code)
		}
		if _, err := orig.FetchMeta(context.Background(), code); err == nil {
			t.Fatalf("FetchMeta(%q): expected error", code)
		}
	}
	if requests != 0 {
		t.Fatalf("upstream requests = %d, want 0 (empty code rejected before network)", requests)
	}
}

// newEastmoneyFundForTest creates an EastmoneyFund that uses the test server as upstream.
// We use a minimal wrapper to override the URL construction.
func newEastmoneyFundForTest(server *httptest.Server) *EastmoneyFund {
	return &EastmoneyFund{
		client:    server.Client(),
		serverURL: server.URL,
	}
}

func TestFetchHistory_MissingEquityReturnDefaultsToZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var Data_netWorthTrend = [{"x":1782864000000,"y":1.2345}];`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	points, err := orig.FetchHistory(context.Background(), "019173")
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("len(points)=%d want 1", len(points))
	}
	if points[0].ChangePct != 0 {
		t.Fatalf("ChangePct=%f, want 0 when equityReturn is absent", points[0].ChangePct)
	}
}

func TestFetchHistory_MalformedSeriesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var Data_netWorthTrend = [{"x":"not-a-number","y":1.2}];`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	points, err := orig.FetchHistory(context.Background(), "019173")
	if err == nil {
		t.Fatalf("expected error for malformed series JSON, got points=%v", points)
	}
	if points != nil {
		t.Fatalf("expected nil points on parse failure, got %v", points)
	}
}

func TestFetchHistory_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	if _, err := orig.FetchHistory(context.Background(), "019173"); err == nil {
		t.Fatal("expected error for upstream HTTP 500")
	}
}

func TestFetchMeta_MissingNameOrType(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"name only", `var fS_name = "测试基金";`},
		{"type only", `var fS_code = "QDII";`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tc.body))
			}))
			defer server.Close()

			orig := newEastmoneyFundForTest(server)
			meta, err := orig.FetchMeta(context.Background(), "019173")
			if err != nil {
				t.Fatalf("FetchMeta: %v", err)
			}
			if meta != nil {
				t.Fatalf("expected nil meta when a required field is missing, got %+v", meta)
			}
		})
	}
}

func TestFetchMeta_MissingInception(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`var fS_name = "测试基金";var fS_code = "QDII";`))
	}))
	defer server.Close()

	orig := newEastmoneyFundForTest(server)
	meta, err := orig.FetchMeta(context.Background(), "019173")
	if err != nil {
		t.Fatalf("FetchMeta: %v", err)
	}
	if meta == nil {
		t.Fatal("expected meta, got nil")
	}
	if meta.Inception != "" {
		t.Fatalf("Inception=%q, want empty when fS_buyMinDate is absent", meta.Inception)
	}
}
