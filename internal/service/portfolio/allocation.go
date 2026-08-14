package portfolio

import (
	"context"

	"fmt"

	"sort"

	"strconv"

	"strings"
)

type AllocationBucket struct {
	Key string `json:"key"`

	Label string `json:"label"`

	Value float64 `json:"value"`

	WeightPct float64 `json:"weight_pct"`

	Count int `json:"count"`
}

type Allocation struct {
	TotalValue float64 `json:"total_value"`

	BySecurityType []AllocationBucket `json:"by_security_type"`

	ByMarket []AllocationBucket `json:"by_market"`

	ByFundType []AllocationBucket `json:"by_fund_type"`

	RiskFlags []string `json:"risk_flags"`

	AgentBrief string `json:"agent_brief"`
}

type allocationLabel struct {
	Key string

	Label string
}

type allocationRow struct {
	Key string

	Label string

	Value float64

	Count int
}

// securityTypeLabels / marketLabels: EN-primary API labels (#180).

// SPA maps by key via allocation.* i18n for locale display.

var securityTypeLabels = map[string]allocationLabel{

	"fund": {Key: "fund", Label: "Fund"},

	"stock": {Key: "stock", Label: "Stock"},

	"etf": {Key: "etf", Label: "ETF"},

	"index": {Key: "index", Label: "Index"},
}

var marketLabels = map[string]allocationLabel{

	"CN": {Key: "cn_fund", Label: "CN Funds"},

	"SH": {Key: "a_share_sh", Label: "A-Share SH"},

	"SZ": {Key: "a_share_sz", Label: "A-Share SZ"},

	"HK": {Key: "hk_stock", Label: "HK Stocks"},

	"US": {Key: "us_stock", Label: "US Stocks"},

	"": {Key: "unclassified", Label: "Unclassified"},
}

func (s Service) GetAllocation(ctx context.Context, portfolioID int) (*Allocation, error) {
	portfolioID = clampPortfolioID(portfolioID)

	totalValue, err := s.allocationTotalValue(ctx, portfolioID)

	if err != nil {

		return nil, err

	}

	byType, err := s.allocationBuckets(ctx, portfolioID, allocationByTypeSQL, securityTypeLabels)

	if err != nil {

		return nil, err

	}

	byMarket, err := s.allocationBuckets(ctx, portfolioID, allocationByMarketSQL, marketLabels)

	if err != nil {

		return nil, err

	}

	byFundType, err := s.allocationBuckets(ctx, portfolioID, allocationByFundTypeSQL, nil)

	if err != nil {

		return nil, err

	}

	allocation := &Allocation{

		TotalValue: round2(totalValue),

		BySecurityType: bucketizeAllocationRows(byType, totalValue, securityTypeLabels),

		ByMarket: bucketizeAllocationRows(byMarket, totalValue, marketLabels),

		ByFundType: bucketizeAllocationRows(byFundType, totalValue, nil),
	}

	allocation.RiskFlags = allocationRiskFlags(allocation)

	allocation.AgentBrief = allocationAgentBrief(allocation)

	return allocation, nil

}

func (s Service) allocationTotalValue(ctx context.Context, portfolioID int) (float64, error) {

	var totalValue float64

	if err := s.db.QueryRowContext(ctx, `

		SELECT COALESCE(SUM(current_value), 0)

		FROM portfolio_snapshot

		WHERE held_shares > 0.001 AND COALESCE(portfolio_id, 1) = ?

	`, portfolioID).Scan(&totalValue); err != nil {

		return 0, fmt.Errorf("sum allocation total value: %w", err)

	}

	return totalValue, nil

}

func (s Service) allocationBuckets(ctx context.Context, portfolioID int, query string, labels map[string]allocationLabel) ([]allocationRow, error) {

	rows, err := s.db.QueryContext(ctx, query, portfolioID)

	if err != nil {

		return nil, fmt.Errorf("query allocation buckets: %w", err)

	}

	defer rows.Close()

	var buckets []allocationRow

	for rows.Next() {

		var row allocationRow

		if err := rows.Scan(&row.Key, &row.Value, &row.Count); err != nil {

			return nil, fmt.Errorf("scan allocation bucket: %w", err)

		}

		row.Key = clampPortfolioText(row.Key, 64)

		if label, ok := labels[row.Key]; ok {

			row.Label = clampPortfolioText(label.Label, 120)

		} else if row.Key == "" {

			row.Label = "Unclassified"

		} else {

			row.Label = row.Key

		}

		buckets = append(buckets, row)

	}

	if err := rows.Err(); err != nil {

		return nil, fmt.Errorf("allocation bucket rows: %w", err)

	}

	return buckets, nil

}

const allocationByTypeSQL = `

	SELECT

		COALESCE(ps.security_type, fd.security_type, 'fund') as key,

		COALESCE(SUM(ps.current_value), 0) as value,

		COUNT(*) as count

	FROM portfolio_snapshot ps

	LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code

	WHERE ps.held_shares > 0.001 AND COALESCE(ps.portfolio_id, 1) = ?

	GROUP BY key

		LIMIT 5000

`

const allocationByMarketSQL = `

	SELECT

		COALESCE(

			NULLIF(fd.market, ''),

			CASE WHEN COALESCE(ps.security_type, fd.security_type, 'fund') = 'fund' THEN 'CN' ELSE '' END

		) as key,

		COALESCE(SUM(ps.current_value), 0) as value,

		COUNT(*) as count

	FROM portfolio_snapshot ps

	LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code

	WHERE ps.held_shares > 0.001 AND COALESCE(ps.portfolio_id, 1) = ?

	GROUP BY key

		LIMIT 5000

`

const allocationByFundTypeSQL = `

	SELECT

		COALESCE(NULLIF(fd.fund_type, ''), 'unclassified') as key,

		COALESCE(SUM(ps.current_value), 0) as value,

		COUNT(*) as count

	FROM portfolio_snapshot ps

	LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code

	WHERE ps.held_shares > 0.001 AND COALESCE(ps.portfolio_id, 1) = ?

	GROUP BY key

		LIMIT 5000

`

func bucketizeAllocationRows(rows []allocationRow, totalValue float64, labels map[string]allocationLabel) []AllocationBucket {

	buckets := make([]AllocationBucket, 0, len(rows))

	for _, row := range rows {

		if row.Value <= 0 {

			continue

		}

		key := row.Key

		label := row.Label

		if entry, ok := labels[row.Key]; ok {

			key = entry.Key

			label = entry.Label

		}

		if key == "" {

			key = "unclassified"

		}

		if label == "" {

			label = key

		}

		weightPct := 0.0

		if totalValue > 0 {

			weightPct = round2(row.Value / totalValue * 100)

		}

		buckets = append(buckets, AllocationBucket{

			Key: key,

			Label: label,

			Value: round2(row.Value),

			WeightPct: weightPct,

			Count: row.Count,
		})

	}

	sort.SliceStable(buckets, func(i, j int) bool {

		if buckets[i].Value == buckets[j].Value {

			return buckets[i].Key < buckets[j].Key

		}

		return buckets[i].Value > buckets[j].Value

	})

	return buckets

}

// allocationRiskFlags returns EN-primary concentration facts for SPA + MCP (#182).
func allocationRiskFlags(allocation *Allocation) []string {
	var flags []string
	if bucketWeight(allocation.BySecurityType, "stock") > 80 {
		flags = append(flags, "Stock weight above 80%")
	}
	if len(allocation.ByMarket) > 0 && allocation.ByMarket[0].WeightPct > 70 {
		flags = append(flags, fmt.Sprintf("%s weight above 70%%", allocation.ByMarket[0].Label))
	}
	if len(allocation.ByFundType) > 0 && allocation.ByFundType[0].WeightPct > 50 {
		flags = append(flags, fmt.Sprintf("%s theme weight above 50%%", allocation.ByFundType[0].Label))
	}
	return flags
}

// allocationAgentBrief is EN-primary facts-only copy (#182). SPA may still locale-map bucket rows by key.
func allocationAgentBrief(allocation *Allocation) string {
	typeBrief := allocationBrief(allocation.BySecurityType)
	if typeBrief == "" {
		typeBrief = "no holdings"
	}
	marketBrief := allocationBrief(limitAllocationBuckets(allocation.ByMarket, 3))
	brief := fmt.Sprintf("Allocation: %s", typeBrief)
	if marketBrief != "" {
		brief += fmt.Sprintf("; Market: %s", marketBrief)
	}
	if len(allocation.RiskFlags) > 0 {
		brief += fmt.Sprintf(". Risk: %s.", strings.Join(allocation.RiskFlags, "; "))
	} else {
		brief += ". Risk: no concentration alerts."
	}
	return brief
}

func bucketWeight(buckets []AllocationBucket, key string) float64 {

	for _, bucket := range buckets {

		if bucket.Key == key {

			return bucket.WeightPct

		}

	}

	return 0

}

func allocationBrief(buckets []AllocationBucket) string {

	parts := make([]string, 0, len(buckets))

	for _, bucket := range buckets {

		parts = append(parts, fmt.Sprintf("%s %s%%", bucket.Label, formatAllocationPct(bucket.WeightPct)))

	}

	return strings.Join(parts, ", ")

}

func limitAllocationBuckets(buckets []AllocationBucket, limit int) []AllocationBucket {

	if len(buckets) <= limit {

		return buckets

	}

	return buckets[:limit]

}

func formatAllocationPct(value float64) string {

	return strconv.FormatFloat(value, 'f', -1, 64)

}
