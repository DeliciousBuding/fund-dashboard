package datasource

import "testing"

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
