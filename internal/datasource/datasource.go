// Package datasource defines the interface for fetching security price data
// from external providers. Implementations handle the transport and parsing
// details; consumers depend only on the interface.
package datasource

import "context"

// PricePoint is a single price/NAV observation from any data source.
type PricePoint struct {
	Date      string  `json:"date"`
	Price     float64 `json:"price"`      // unit_nav for funds, close for stocks
	ChangePct float64 `json:"change_pct"` // daily change in percent
}

// FundMeta is basic fund identity fetched from the data source.
type FundMeta struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Inception string `json:"inception"`
}

// PriceSource fetches price history for a security. Each implementation
// knows how to reach its upstream API and how to parse the response.
type PriceSource interface {
	// FetchHistory returns all available price history for the given code.
	// Returns an empty slice (not nil) when the security has no data.
	FetchHistory(ctx context.Context, code string) ([]PricePoint, error)

	// FetchMeta returns basic fund/stock metadata. May return nil when
	// the source does not have metadata for this code.
	FetchMeta(ctx context.Context, code string) (*FundMeta, error)
}

// SecurityType classifies a security for routing to the correct source.
type SecurityType string

const (
	TypeFund  SecurityType = "fund"
	TypeStock SecurityType = "stock"
)

// HeldSecurity is a security currently held in a portfolio.
type HeldSecurity struct {
	Code string
	Type SecurityType
}
