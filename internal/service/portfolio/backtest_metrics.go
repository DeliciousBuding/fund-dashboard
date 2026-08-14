package portfolio

import (
	"math"
	"time"
)

type backtestMetrics struct {
	TotalReturnPct  float64
	AnnualReturnPct float64
	MaxDrawdownPct  float64
	SharpeRatio     float64
}

func computeBacktestMetrics(timeline []BacktestPoint, totalInvested float64, startDate string, endDate string) backtestMetrics {
	if len(timeline) == 0 {
		return backtestMetrics{}
	}
	peak := math.Inf(-1)
	maxDrawdown := 0.0
	returns := []float64{}
	for i, point := range timeline {
		if point.TotalValue > peak {
			peak = point.TotalValue
		}
		if peak > 0 {
			maxDrawdown = math.Max(maxDrawdown, (peak-point.TotalValue)/peak)
		}
		if i > 0 && timeline[i-1].TotalValue > 0 {
			returns = append(returns, (point.TotalValue-timeline[i-1].TotalValue)/timeline[i-1].TotalValue)
		}
	}
	totalReturn := 0.0
	if totalInvested > 0 {
		totalReturn = (timeline[len(timeline)-1].TotalValue - totalInvested) / totalInvested
	}
	years := math.Max(yearsBetween(startDate, endDate), 0.01)
	annualReturn := -1.0
	if 1+totalReturn > 0 {
		annualReturn = math.Pow(1+totalReturn, 1/years) - 1
	}
	return backtestMetrics{
		TotalReturnPct:  round2(totalReturn * 100),
		AnnualReturnPct: round2(annualReturn * 100),
		MaxDrawdownPct:  round2(maxDrawdown * 100),
		SharpeRatio:     round2(backtestSharpe(returns)),
	}
}

func backtestSharpe(returns []float64) float64 {
	mean := 0.0
	for _, value := range returns {
		mean += value
	}
	if len(returns) > 0 {
		mean /= float64(len(returns))
	}
	variance := 0.0
	if len(returns) > 1 {
		for _, value := range returns {
			variance += math.Pow(value-mean, 2)
		}
		variance /= float64(len(returns) - 1)
	}
	if variance <= 0 {
		return 0
	}
	return (mean*252 - 0.02) / (math.Sqrt(variance) * math.Sqrt(252))
}

func yearsBetween(startDate string, endDate string) float64 {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0.01
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return 0.01
	}
	return end.Sub(start).Hours() / 24 / 365.25
}
