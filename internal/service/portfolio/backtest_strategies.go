package portfolio

import (
	"fmt"
	"math"
	"sort"
	"time"
)

func simGridBacktest(navs []backtestNAV, startDate string, baseAmount float64, levels int) ([]BacktestTrade, []BacktestPoint) {
	filtered := navsFrom(navs, startDate)
	if levels <= 0 {
		levels = 5
	}
	trades := []BacktestTrade{}
	timeline := []BacktestPoint{}
	if len(filtered) == 0 {
		return trades, timeline
	}
	calibLen := int(math.Max(float64(len(filtered))*0.2, 5))
	if calibLen > len(filtered) {
		calibLen = len(filtered)
	}
	lo, hi := filtered[0].UnitNAV, filtered[0].UnitNAV
	for _, nav := range filtered[:calibLen] {
		lo = math.Min(lo, nav.UnitNAV)
		hi = math.Max(hi, nav.UnitNAV)
	}
	step := (hi - lo) / float64(levels)
	shares, cash, invested := 0.0, 0.0, 0.0
	prev := -1
	for _, nav := range filtered {
		grid := 0
		if step > 0 {
			grid = int(math.Min(math.Floor((nav.UnitNAV-lo)/step), float64(levels-1)))
		}
		if prev >= 0 && grid != prev {
			delta := prev - grid
			amount := baseAmount * math.Abs(float64(delta))
			if delta > 0 {
				bought := amount / nav.UnitNAV
				shares += bought
				cash -= amount
				invested += amount
				trades = append(trades, BacktestTrade{Date: nav.Date, Action: "buy", Price: round4(nav.UnitNAV), Shares: round4(bought), Amount: round2(amount), Reason: fmt.Sprintf("价格下跌至第%d格(从%d)，买入", grid+1, prev+1)})
			} else {
				sold := math.Min(amount/nav.UnitNAV, shares)
				if sold > 0.0001 {
					shares -= sold
					cash += sold * nav.UnitNAV
					trades = append(trades, BacktestTrade{Date: nav.Date, Action: "sell", Price: round4(nav.UnitNAV), Shares: round4(sold), Amount: round2(sold * nav.UnitNAV), Reason: fmt.Sprintf("价格上涨至第%d格(从%d)，卖出", grid+1, prev+1)})
				}
			}
		}
		prev = grid
		timeline = append(timeline, backtestPoint(nav, shares, cash, invested))
	}
	return trades, timeline
}

func simMomentumBacktest(navs []backtestNAV, startDate string, baseAmount float64, lookback int) ([]BacktestTrade, []BacktestPoint) {
	filtered := navsFrom(navs, startDate)
	trades := []BacktestTrade{}
	timeline := []BacktestPoint{}
	if len(filtered) == 0 {
		return trades, timeline
	}
	if lookback <= 0 {
		lookback = 3
	}
	shares := baseAmount / filtered[0].UnitNAV
	cash := -baseAmount
	invested := baseAmount
	trades = append(trades, BacktestTrade{Date: filtered[0].Date, Action: "buy", Price: round4(filtered[0].UnitNAV), Shares: round4(shares), Amount: round2(baseAmount), Reason: "动量策略初始建仓"})
	lastYear, lastMonth := -1, -1
	for _, nav := range filtered {
		year, month := yearMonth(nav.Date)
		if year != lastYear || month != lastMonth {
			lastYear, lastMonth = year, month
			if past, ok := momentumPastNAV(navs, startDate, nav.Date, lookback); ok && past > 0 {
				momentum := (nav.UnitNAV - past) / past
				if momentum > 0.02 {
					bought := baseAmount / nav.UnitNAV
					shares += bought
					cash -= baseAmount
					invested += baseAmount
					trades = append(trades, BacktestTrade{Date: nav.Date, Action: "buy", Price: round4(nav.UnitNAV), Shares: round4(bought), Amount: round2(baseAmount), Reason: fmt.Sprintf("%d月动量+%.1f%%，买入", lookback, momentum*100)})
				} else if momentum < -0.02 {
					sold := math.Min(baseAmount/nav.UnitNAV, shares)
					if sold > 0.0001 {
						shares -= sold
						cash += sold * nav.UnitNAV
						trades = append(trades, BacktestTrade{Date: nav.Date, Action: "sell", Price: round4(nav.UnitNAV), Shares: round4(sold), Amount: round2(sold * nav.UnitNAV), Reason: fmt.Sprintf("%d月动量%.1f%%，卖出", lookback, momentum*100)})
					}
				}
			}
		}
		timeline = append(timeline, backtestPoint(nav, shares, cash, invested))
	}
	return trades, timeline
}

func simRebalanceBacktest(navs []backtestNAV, startDate string, baseAmount float64, targetWeight float64, interval int) ([]BacktestTrade, []BacktestPoint) {
	filtered := navsFrom(navs, startDate)
	trades := []BacktestTrade{}
	timeline := []BacktestPoint{}
	if len(filtered) == 0 {
		return trades, timeline
	}
	if targetWeight <= 0 || targetWeight > 1 {
		targetWeight = 0.6
	}
	if interval <= 0 {
		interval = 3
	}
	initialEquity := baseAmount * targetWeight
	shares := initialEquity / filtered[0].UnitNAV
	cash := baseAmount - initialEquity
	invested := baseAmount
	trades = append(trades, BacktestTrade{Date: filtered[0].Date, Action: "buy", Price: round4(filtered[0].UnitNAV), Shares: round4(shares), Amount: round2(initialEquity), Reason: "再平衡初始建仓"})
	startYear, startMonth := yearMonth(filtered[0].Date)
	lastYear, lastMonth := -1, -1
	for _, nav := range filtered {
		year, month := yearMonth(nav.Date)
		months := (year-startYear)*12 + (month - startMonth)
		if months > 0 && months%interval == 0 && (year != lastYear || month != lastMonth) {
			lastYear, lastMonth = year, month
			equity := shares * nav.UnitNAV
			total := equity + cash
			target := total * targetWeight
			diff := target - equity
			if math.Abs(diff) > baseAmount*0.1 {
				if diff > 0 {
					bought := diff / nav.UnitNAV
					shares += bought
					cash -= diff
					invested += diff
					trades = append(trades, BacktestTrade{Date: nav.Date, Action: "buy", Price: round4(nav.UnitNAV), Shares: round4(bought), Amount: round2(diff), Reason: fmt.Sprintf("再平衡:权益不足%.0f%%，补仓", targetWeight*100)})
				} else {
					sold := math.Min(math.Abs(diff)/nav.UnitNAV, shares)
					if sold > 0.0001 {
						shares -= sold
						cash += sold * nav.UnitNAV
						trades = append(trades, BacktestTrade{Date: nav.Date, Action: "sell", Price: round4(nav.UnitNAV), Shares: round4(sold), Amount: round2(sold * nav.UnitNAV), Reason: fmt.Sprintf("再平衡:权益超出%.0f%%，减仓", targetWeight*100)})
					}
				}
			}
		}
		timeline = append(timeline, backtestPoint(nav, shares, cash, invested))
	}
	return trades, timeline
}

func lumpSumBacktestReturn(navs []backtestNAV, startDate string, invested float64) BacktestReturn {
	filtered := navsFrom(navs, startDate)
	if len(filtered) == 0 {
		return BacktestReturn{Invested: round2(invested), FinalValue: 0, ReturnPct: -100}
	}
	shares := invested / filtered[0].UnitNAV
	finalValue := shares * filtered[len(filtered)-1].UnitNAV
	return BacktestReturn{Invested: round2(invested), FinalValue: round2(finalValue), ReturnPct: round2((finalValue - invested) / invested * 100)}
}

func dcaBacktestReturn(navs []backtestNAV, startDate string, baseAmount float64) BacktestReturn {
	filtered := navsFrom(navs, startDate)
	if len(filtered) == 0 {
		return BacktestReturn{}
	}
	shares, invested := 0.0, 0.0
	lastYear, lastMonth := -1, -1
	for _, nav := range filtered {
		year, month := yearMonth(nav.Date)
		if year != lastYear || month != lastMonth {
			lastYear, lastMonth = year, month
			shares += baseAmount / nav.UnitNAV
			invested += baseAmount
		}
	}
	finalValue := shares * filtered[len(filtered)-1].UnitNAV
	return BacktestReturn{Invested: round2(invested), FinalValue: round2(finalValue), ReturnPct: round2((finalValue - invested) / invested * 100)}
}

func backtestPoint(nav backtestNAV, shares float64, cash float64, invested float64) BacktestPoint {
	equity := shares * nav.UnitNAV
	return BacktestPoint{Date: nav.Date, NAV: round4(nav.UnitNAV), SharesHeld: round4(shares), Cash: round2(cash), EquityValue: round2(equity), TotalValue: round2(equity + cash), TotalInvested: round2(invested)}
}

func navsFrom(navs []backtestNAV, startDate string) []backtestNAV {
	filtered := make([]backtestNAV, 0, len(navs))
	for _, nav := range navs {
		if nav.Date >= startDate {
			filtered = append(filtered, nav)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Date < filtered[j].Date })
	return filtered
}

func momentumPastNAV(navs []backtestNAV, startDate string, currentDate string, lookbackMonths int) (float64, bool) {
	current, err := time.Parse("2006-01-02", currentDate)
	if err != nil {
		return 0, false
	}
	lookback := time.Date(current.Year(), current.Month()-time.Month(lookbackMonths), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	sorted := navsFrom(navs, "")
	for _, nav := range sorted {
		if nav.Date >= lookback && nav.Date < currentDate {
			return nav.UnitNAV, true
		}
	}
	for _, nav := range sorted {
		if nav.Date >= startDate {
			return nav.UnitNAV, true
		}
	}
	return 0, false
}

func normalizeBacktestStrategy(strategy string) string {
	switch strategy {
	case "grid", "momentum", "rebalance", "dca":
		return strategy
	default:
		return "dca"
	}
}

func yearMonth(date string) (int, int) {
	if len(date) < 7 {
		return 0, 0
	}
	t, err := time.Parse("2006-01", date[:7])
	if err != nil {
		return 0, 0
	}
	return t.Year(), int(t.Month())
}
