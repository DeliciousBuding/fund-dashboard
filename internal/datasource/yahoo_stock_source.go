package datasource

import (
	"context"
	"fmt"
	"strings"
)

// YahooStock is a PriceSource for US equities via Yahoo chart history.
type YahooStock struct{}

// Compile-time check.
var _ PriceSource = (*YahooStock)(nil)

func NewYahooStock() *YahooStock { return &YahooStock{} }

func (s *YahooStock) FetchHistory(ctx context.Context, code string) ([]PricePoint, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, fmt.Errorf("stock code is required")
	}
	snap, err := FetchYahooStockSnapshot(ctx, code, "1y", true)
	if err != nil {
		return nil, err
	}
	out := make([]PricePoint, 0, len(snap.History))
	for _, h := range snap.History {
		if h.Close <= 0 || h.Date == "" {
			continue
		}
		out = append(out, PricePoint{Date: h.Date, Price: h.Close, ChangePct: h.ChangePct})
	}
	// If history empty but quote price present, emit a single point.
	if len(out) == 0 && snap.Price > 0 {
		d := snap.MarketTime.UTC().Format("2006-01-02")
		if d != "" && d != "0001-01-01" {
			out = append(out, PricePoint{Date: d, Price: snap.Price, ChangePct: snap.ChangePct})
		}
	}
	if out == nil {
		out = []PricePoint{}
	}
	return out, nil
}

func (s *YahooStock) FetchMeta(ctx context.Context, code string) (*FundMeta, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, fmt.Errorf("stock code is required")
	}
	snap, err := FetchYahooStockSnapshot(ctx, code, "5d", false)
	if err != nil {
		return nil, err
	}
	return &FundMeta{Code: code, Name: snap.Name, Type: string(TypeStock)}, nil
}
