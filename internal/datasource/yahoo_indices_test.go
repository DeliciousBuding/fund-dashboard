package datasource

import "testing"

func TestNormalizeIndexSymbolCNHK(t *testing.T) {
	tests := []struct{ raw, want string }{
		// US (existing)
		{"NDX", "^NDX"},
		{"^NDX", "^NDX"},
		{"GSPC", "^GSPC"},
		{"^GSPC", "^GSPC"},
		// HK
		{"HSI", "^HSI"},
		{"^HSI", "^HSI"},
		// CN via SPA codes
		{"sh000001", "000001.SS"},
		{"SH000001", "000001.SS"},
		{"sz399001", "399001.SZ"},
		{"SZ399001", "399001.SZ"},
		{"sz399006", "399006.SZ"},
		{"SZ399006", "399006.SZ"},
		// CN via direct Yahoo codes (passthrough after upper)
		{"000001.SS", "000001.SS"},
		{"399001.SZ", "399001.SZ"},
		{"399006.SZ", "399006.SZ"},
	}
	for _, tc := range tests {
		got := NormalizeIndexSymbol(tc.raw)
		if got != tc.want {
			t.Errorf("NormalizeIndexSymbol(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestIndexMarketRegions(t *testing.T) {
	tests := []struct{ symbol, market string }{
		{"^NDX", "US"},
		{"^GSPC", "US"},
		{"^DJI", "US"},
		{"^IXIC", "US"},
		{"^HSI", "HK"},
		{"000001.SS", "CN"},
		{"399001.SZ", "CN"},
		{"399006.SZ", "CN"},
	}
	for _, tc := range tests {
		got := indexMarket(tc.symbol)
		if got != tc.market {
			t.Errorf("indexMarket(%q) = %q, want %q", tc.symbol, got, tc.market)
		}
	}
}

func TestDefaultIndexSymbolsIncludesCNHK(t *testing.T) {
	found := map[string]bool{}
	for _, s := range DefaultIndexSymbols {
		found[s] = true
	}
	for _, want := range []string{"^HSI", "000001.SS", "399001.SZ", "399006.SZ"} {
		if !found[want] {
			t.Errorf("DefaultIndexSymbols missing %q", want)
		}
	}
	if len(DefaultIndexSymbols) != 8 {
		t.Errorf("DefaultIndexSymbols len = %d, want 8", len(DefaultIndexSymbols))
	}

}

func TestDefaultIndexNamesHasCNHK(t *testing.T) {
	for _, code := range []string{"^HSI", "000001.SS", "399001.SZ", "399006.SZ", "^NDX", "^GSPC", "^DJI", "^IXIC"} {
		name, ok := defaultIndexNames[code]
		if !ok || name == "" {
			t.Errorf("defaultIndexNames[%q] missing or empty: %q", code, name)
			continue
		}
		// EN-primary fallback (#164) — must not be pure Chinese display names.
		for _, r := range name {
			if r >= 0x4e00 && r <= 0x9fff {
				t.Errorf("defaultIndexNames[%q]=%q contains CJK; want English Yahoo-style fallback", code, name)
				break
			}
		}
	}
}

func TestNormalizeIndexSymbolBoundsLength(t *testing.T) {
	long := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789extra"
	got := NormalizeIndexSymbol(long)
	if len(got) != 32 {
		t.Fatalf("len=%d got=%q", len(got), got)
	}
}

func TestNormalizeYahooRangeDefaultsUnknown(t *testing.T) {
	if got := NormalizeYahooRange("not-a-range"); got != "1y" {
		t.Fatalf("unknown range = %q, want 1y", got)
	}
	if got := NormalizeYahooRange(string(make([]byte, 40))); got != "1y" {
		t.Fatalf("long range = %q, want 1y", got)
	}
	if got := NormalizeYahooRange("3mo"); got != "3mo" {
		t.Fatalf("3mo = %q", got)
	}
	if got := NormalizeYahooInterval("bogus"); got != "1d" {
		t.Fatalf("interval default = %q", got)
	}
}
