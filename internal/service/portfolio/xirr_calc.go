package portfolio

import (
	"math"
	"database/sql"
	"fmt"
	"time"
)

func scanXIRRTransactions(rows *sql.Rows) ([]xirrTransaction, error) {
	transactions := []xirrTransaction{}
	for rows.Next() {
		var tx xirrTransaction
		var amount sql.NullFloat64
		var direction sql.NullString
		var tradeTime string
		var fee sql.NullFloat64
		if err := rows.Scan(&amount, &direction, &tradeTime, &fee); err != nil {
			return nil, fmt.Errorf("scan xirr transaction: %w", err)
		}
		parsed, err := parseXIRRTime(tradeTime)
		if err != nil {
			return nil, fmt.Errorf("parse xirr trade time: %w", err)
		}
		if amount.Valid {
			tx.Amount = amount.Float64
		}
		if direction.Valid {
			tx.Direction = direction.String
		}
		if fee.Valid {
			tx.Fee = fee.Float64
		}
		tx.TradeTime = parsed
		transactions = append(transactions, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("xirr transaction rows: %w", err)
	}
	return transactions, nil
}

func buildXIRRCashflows(transactions []xirrTransaction, currentValue float64) []xirrCashflow {
	if len(transactions) == 0 {
		return nil
	}
	lastTime := transactions[len(transactions)-1].TradeTime
	cashflows := make([]xirrCashflow, 0, len(transactions)+1)
	for _, tx := range transactions {
		amount := 0.0
		switch tx.Direction {
		case "buy":
			amount = -(tx.Amount + tx.Fee)
		case "sell":
			amount = tx.Amount - tx.Fee
		case "dividend":
			amount = tx.Amount
		}
		cashflows = append(cashflows, xirrCashflow{
			Amount: amount,
			Years:  lastTime.Sub(tx.TradeTime).Hours() / 24 / 365,
		})
	}
	if currentValue > 0 {
		cashflows = append(cashflows, xirrCashflow{Amount: currentValue})
	}
	return cashflows
}

func calcXIRR(cashflows []xirrCashflow) *float64 {
	if len(cashflows) < 2 {
		return nil
	}
	hasPositive := false
	hasNegative := false
	for _, cashflow := range cashflows {
		if cashflow.Amount > 0 {
			hasPositive = true
		}
		if cashflow.Amount < 0 {
			hasNegative = true
		}
	}
	if !hasPositive || !hasNegative {
		return nil
	}

	npv := func(rate float64) float64 {
		total := 0.0
		for _, cashflow := range cashflows {
			years := math.Max(cashflow.Years, 1e-10)
			total += cashflow.Amount * math.Pow(1+rate, years)
		}
		return total
	}

	for _, guess := range []float64{0.1, 0.3, 0.5, 0.7, 0.9, -0.3, -0.5} {
		rate := guess
		previous := math.Inf(1)
		for i := 0; i < 80; i++ {
			value := npv(rate)
			if math.Abs(value) < 0.001 {
				return &rate
			}
			derivative := (npv(rate+1e-6) - value) / 1e-6
			if math.Abs(derivative) < 1e-14 {
				break
			}
			next := rate - value/derivative
			if math.Abs(next-previous) < 1e-9 {
				return &next
			}
			previous = rate
			rate = math.Max(-0.999, math.Min(next, 1e6))
		}
	}

	if rate := bisectXIRR(npv, 10); rate != nil {
		return rate
	}
	if rate := bisectXIRR(npv, 1000); rate != nil {
		return rate
	}
	if npv(0) > 0.01 && npv(1000) > 0 {
		return nil
	}
	return nil
}

func bisectXIRR(npv func(float64) float64, high float64) *float64 {
	low := -0.999
	for i := 0; i < 200; i++ {
		mid := (low + high) / 2
		value := npv(mid)
		if math.Abs(value) < 0.001 {
			return &mid
		}
		if npv(low)*value < 0 {
			high = mid
		} else {
			low = mid
		}
	}
	return nil
}

func parseXIRRTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time %q", value)
}
