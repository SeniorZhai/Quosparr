package usage

import (
	"math"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/usage/decimal"
)

func TestCostOverviewUsesLocalCalendarDaysAndExactAmounts(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 3, 1, 0, 0, 0, loc)
	var daily []DailyRow
	for _, row := range []struct {
		days   int
		tokens int64
		cost   string
	}{{0, 10, "0.1"}, {-6, 20, "0.2"}, {-7, 30, "0.3"}, {-29, 40, "0.4"}, {-30, 50, "10"}, {1, 60, "20"}} {
		amount := decimal.MustParse(row.cost)
		daily = append(daily, DailyRow{Date: now.AddDate(0, 0, row.days).Format(time.DateOnly), Totals: Totals{TotalTokens: row.tokens}, EstimatedUSD: &amount})
	}
	second := decimal.MustParse("0.2")
	got := BuildCostOverview(now.UTC(), loc, []Summary{{DailyRows: daily}, {DailyRows: []DailyRow{{Date: now.Format(time.DateOnly), Totals: Totals{TotalTokens: 5}, EstimatedUSD: &second}}}}, false)
	for i, want := range []struct {
		tokens int64
		amount string
	}{{15, "0.3"}, {35, "0.5"}, {105, "1.2"}} {
		period := got.Periods[i]
		if period.Tokens != want.tokens || period.EstimatedUSD == nil || period.EstimatedUSD.String() != want.amount || period.Partial {
			t.Fatalf("period %d = %+v, want %+v", i, period, want)
		}
	}
	if !got.CalculatedAt.Equal(now) || second.String() != "0.2" || daily[0].EstimatedUSD.String() != "0.1" {
		t.Fatal("overview changed its source or calculation timestamp")
	}
}

func TestCostOverviewDistinguishesFreeUnknownAndMissingUsage(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	zero := decimal.Zero
	daily := []DailyRow{
		{Date: "2026-10-28", Totals: Totals{TotalTokens: 10}, EstimatedUSD: &zero},
		{Date: "2026-10-21", Totals: Totals{TotalTokens: 20}, UnpricedTokens: 20},
	}
	got := BuildCostOverview(now, time.UTC, []Summary{{DailyRows: daily, Warnings: []string{"unknown-model:test", "price-source:models.dev:test", "usage-index-rebuilt:parser-version"}}}, false)
	for i, period := range got.Periods {
		if period.EstimatedUSD == nil || !period.EstimatedUSD.IsZero() || period.Partial != (i == 2) {
			t.Fatalf("period %d = %+v", i, period)
		}
	}
	unknown := BuildCostOverview(now, time.UTC, []Summary{{DailyRows: daily[1:]}}, false)
	if unknown.Periods[2].EstimatedUSD != nil || !unknown.Periods[2].Partial || unknown.Periods[0].Partial {
		t.Fatalf("unknown = %+v", unknown)
	}
	for _, period := range BuildCostOverview(now, time.UTC, nil, false).Periods {
		if period.EstimatedUSD != nil || period.Tokens != 0 || period.Partial {
			t.Fatalf("empty = %+v", period)
		}
	}
}

func TestCostOverviewMarksMissingRecordsAndOverflow(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	for _, warning := range []string{"malformed-jsonl-lines:2", "oversized-jsonl-lines:1"} {
		got := BuildCostOverview(now, time.UTC, []Summary{{Warnings: []string{warning}}}, false)
		for _, period := range got.Periods {
			if !period.Partial {
				t.Fatalf("%s missing partial flag", warning)
			}
		}
	}
	for _, period := range BuildCostOverview(now, time.UTC, nil, true).Periods {
		if !period.Partial {
			t.Fatal("failed provider did not mark the total partial")
		}
	}
	got := BuildCostOverview(now, time.UTC, []Summary{{DailyRows: []DailyRow{
		{Date: "2026-10-28", Totals: Totals{TotalTokens: math.MaxInt64}},
		{Date: "2026-10-28", Totals: Totals{TotalTokens: 1}},
	}}}, false)
	if got.Periods[0].Tokens != math.MaxInt64 || !got.Periods[0].Partial {
		t.Fatalf("overflow = %+v", got.Periods[0])
	}
}
