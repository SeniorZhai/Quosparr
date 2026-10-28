package usage

import (
	"math"
	"strings"
	"time"

	"github.com/licoy/quosparr/internal/usage/decimal"
)

type CostOverviewPeriod struct {
	EstimatedUSD *decimal.Decimal
	Partial      bool
	Tokens       int64
}

// CostOverview covers today, seven days and thirty days, including today.
type CostOverview struct {
	Periods      [3]CostOverviewPeriod
	CalculatedAt time.Time
}

// BuildCostOverview combines local daily summaries without estimating unpriced usage.
func BuildCostOverview(now time.Time, loc *time.Location, summaries []Summary, incomplete bool) CostOverview {
	if loc == nil {
		loc = time.Local
	}
	today := Today(now, loc).Window.Start
	starts := [3]string{today.Format(time.DateOnly), today.AddDate(0, 0, -6).Format(time.DateOnly), today.AddDate(0, 0, -29).Format(time.DateOnly)}
	end := today.Format(time.DateOnly)
	out := CostOverview{CalculatedAt: now}
	for _, summary := range summaries {
		for _, warning := range summary.Warnings {
			if strings.HasPrefix(warning, "malformed-jsonl-lines") || strings.HasPrefix(warning, "oversized-jsonl-lines") {
				incomplete = true
			}
		}
		for _, day := range summary.DailyRows {
			for i, start := range starts {
				if day.Date < start || day.Date > end {
					continue
				}
				period := &out.Periods[i]
				if day.Totals.TotalTokens > math.MaxInt64-period.Tokens {
					period.Partial = true
				} else {
					period.Tokens += day.Totals.TotalTokens
				}
				period.Partial = period.Partial || day.UnpricedTokens > 0 || day.Totals.TotalTokens > 0 && day.EstimatedUSD == nil
				if day.EstimatedUSD != nil {
					amount := *day.EstimatedUSD
					if period.EstimatedUSD != nil {
						amount = period.EstimatedUSD.Add(amount)
					}
					if amount.Overflow() {
						period.Partial = true
					} else {
						period.EstimatedUSD = &amount
					}
				}
			}
		}
	}
	if incomplete {
		for i := range out.Periods {
			out.Periods[i].Partial = true
		}
	}
	return out
}
