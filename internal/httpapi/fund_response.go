package httpapi

import (
	"sort"
	"strings"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func fundDetailResponse(detail portfoliosvc.FundDetail) fundDetailJSON {
	totalCost := 0.0
	if detail.Position.CostBasis != nil {
		totalCost = *detail.Position.CostBasis
	}
	return fundDetailJSON{
		Code:             detail.Code,
		Name:             stringPtrValue(detail.Name),
		SecurityType:     detail.SecurityType,
		Market:           detail.Market,
		HeldShares:       detail.Position.Shares,
		TotalCost:        totalCost,
		LatestNAV:        detail.Position.LatestNAV,
		CurrentValue:     detail.Position.MarketValue,
		UnrealizedPNL:    detail.Position.UnrealizedPNL,
		PNLPct:           detail.Position.PNLPct,
		AutoBuyCount:     countTransactions(detail.Transactions, true, "buy"),
		ManualBuyCount:   countTransactions(detail.Transactions, false, "buy"),
		AutoBuyAmount:    sumTransactionAmounts(detail.Transactions, true, "buy"),
		ManualBuyAmount:  sumTransactionAmounts(detail.Transactions, false, "buy"),
		AutoTx:           countTransactions(detail.Transactions, true, ""),
		ManualTx:         countTransactions(detail.Transactions, false, ""),
		BuyCount:         countTransactionsByDirection(detail.Transactions, "buy"),
		SellCount:        countTransactionsByDirection(detail.Transactions, "sell"),
		MedianSettlement: medianSettlementDays(detail.Transactions),
		Transactions:     fundTransactionResponses(detail.Transactions),
	}
}

func fundTransactionResponses(transactions []portfoliosvc.FundTransaction) []fundTransaction {
	out := make([]fundTransaction, 0, len(transactions))
	for _, tx := range transactions {
		seq := tx.Seq
		out = append(out, fundTransaction{
			Seq:            &seq,
			TradeTime:      stringPtrValue(tx.Time),
			ConfirmDate:    tx.ConfirmDate,
			TradeType:      stringPtrValue(tx.Type),
			Direction:      stringPtrValue(tx.Direction),
			Amount:         floatPtrValue(tx.Amount),
			Shares:         floatPtrValue(tx.Shares),
			Fee:            floatPtrValue(tx.Fee),
			NAV:            nil,
			InferredNAV:    nil,
			SettlementDays: tx.SettlementDays,
			OrderID:        tx.OrderID,
			Anomaly:        nil,
		})
	}
	return out
}

func countTransactions(transactions []portfoliosvc.FundTransaction, auto bool, direction string) int {
	count := 0
	for _, tx := range transactions {
		if direction != "" && stringPtrValue(tx.Direction) != direction {
			continue
		}
		if transactionIsAuto(tx) == auto {
			count++
		}
	}
	return count
}

func countTransactionsByDirection(transactions []portfoliosvc.FundTransaction, direction string) int {
	count := 0
	for _, tx := range transactions {
		if stringPtrValue(tx.Direction) == direction {
			count++
		}
	}
	return count
}

func sumTransactionAmounts(transactions []portfoliosvc.FundTransaction, auto bool, direction string) float64 {
	total := 0.0
	for _, tx := range transactions {
		if direction != "" && stringPtrValue(tx.Direction) != direction {
			continue
		}
		if transactionIsAuto(tx) == auto {
			total += floatPtrValue(tx.Amount)
		}
	}
	return total
}

func medianSettlementDays(transactions []portfoliosvc.FundTransaction) int {
	values := []int{}
	for _, tx := range transactions {
		if tx.SettlementDays != nil {
			values = append(values, *tx.SettlementDays)
		}
	}
	if len(values) == 0 {
		return 0
	}
	sort.Ints(values)
	return values[len(values)/2]
}

func transactionIsAuto(tx portfoliosvc.FundTransaction) bool {
	return strings.Contains(stringPtrValue(tx.Type), "定投")
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func floatPtrValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func drawdownResponse(report portfoliosvc.DrawdownReport) drawdownJSON {
	return drawdownJSON{
		Code:        report.Code,
		MaxDrawdown: report.MaxDrawdownPct,
		PeakDate:    report.PeakDate,
		TroughDate:  report.TroughDate,
	}
}

func penetrationResponse(report *portfoliosvc.PenetrationReport) penetrationJSON {
	if report == nil {
		return penetrationJSON{Penetration: []penetrationStockJSON{}}
	}
	rows := make([]penetrationStockJSON, 0, len(report.Penetration))
	for _, row := range report.Penetration {
		rows = append(rows, penetrationStockJSON{
			StockCode:        row.StockCode,
			StockName:        row.StockName,
			TotalExposureCNY: row.EstimatedMarketValueCNY,
			WeightPct:        row.PenetrationPct,
			HeldByFunds:      penetrationFundResponses(row.HeldByFunds),
		})
	}
	return penetrationJSON{
		Penetration:         rows,
		TotalPortfolioValue: report.TotalPortfolioValueCNY,
		EquityFundCount:     report.FundsWithHoldings,
		UniqueStocks:        report.StocksFound,
	}
}

func penetrationFundResponses(rows []portfoliosvc.PenetrationFundExposure) []penetrationFundJSON {
	out := make([]penetrationFundJSON, 0, len(rows))
	for _, row := range rows {
		out = append(out, penetrationFundJSON{
			FundCode:     row.FundCode,
			FundName:     row.FundName,
			WeightPct:    row.WeightPct,
			FundValueCNY: row.FundValueCNY,
		})
	}
	return out
}

type sourceEventJSON struct {
	ID                  int64   `json:"id"`
	Title               string  `json:"title"`
	URL                 *string `json:"url"`
	Source              string  `json:"source"`
	Snippet             *string `json:"snippet"`
	Query               *string `json:"query"`
	RelatedSecurityCode *string `json:"related_security_code"`
	RelatedSecurityName *string `json:"related_security_name"`
	IsRead              bool    `json:"is_read"`
	IsUseful            bool    `json:"is_useful"`
	FetchedAt           string  `json:"fetched_at"`
	CreatedAt           string  `json:"created_at"`
}

func sourceEventResponses(events []portfoliosvc.SourceEvent) []sourceEventJSON {
	out := make([]sourceEventJSON, 0, len(events))
	for _, event := range events {
		out = append(out, sourceEventResponse(event))
	}
	return out
}

func sourceEventResponse(event portfoliosvc.SourceEvent) sourceEventJSON {
	return sourceEventJSON{
		ID:                  event.ID,
		Title:               event.Title,
		URL:                 event.URL,
		Source:              event.Source,
		Snippet:             event.Snippet,
		Query:               event.Query,
		RelatedSecurityCode: event.RelatedSecurityCode,
		RelatedSecurityName: event.RelatedSecurityName,
		IsRead:              event.IsRead != 0,
		IsUseful:            event.IsUseful != 0,
		FetchedAt:           event.FetchedAt,
		CreatedAt:           event.CreatedAt,
	}
}
