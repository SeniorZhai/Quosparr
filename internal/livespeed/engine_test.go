package livespeed

import "testing"
import "time"

func TestOutputRateUsesOnlyTheLastThreeSeconds(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	measured := Response{
		Agent: "a", Source: "grok", Output: 300, Final: true,
		Start: now.Add(-3 * time.Second), End: now,
	}
	if got := OutputRate(now, []Response{measured}, nil); mathAbs(got-100) > 0.01 {
		t.Fatalf("inside window = %v", got)
	}
	old := measured
	old.Start = now.Add(-10 * time.Minute)
	old.End = now.Add(-10*time.Minute + 3*time.Second)
	old.Output = 100000
	if got := OutputRate(now, []Response{old}, nil); got != 0 {
		t.Fatalf("old average leaked into the live rate: %v", got)
	}
}

func TestOutputRateEstimatesAnOpenRequestFromRecentSpeed(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	past := Response{
		Agent: "a", Model: "m", Source: "grok", Output: 100, Final: true,
		Start: now.Add(-2 * time.Minute), End: now.Add(-2*time.Minute + 2*time.Second),
	}
	flight := Flight{Agent: "a", Model: "m", Source: "grok", Start: now.Add(-3 * time.Second), Last: now}
	if got := OutputRate(now, []Response{past}, []Flight{flight}); mathAbs(got-50) > 0.01 {
		t.Fatalf("in flight = %v", got)
	}
	lonely := Flight{Agent: "new", Source: "grok", Start: now.Add(-time.Second), Last: now}
	if got := OutputRate(now, nil, []Flight{lonely}); got != 0 {
		t.Fatalf("no reference still produced %v", got)
	}
}

func TestOutputRateWidensAVeryShortResponse(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	// 50 tokens sit entirely inside the 3s window, so the live rate is 50/3.
	// Widening only changes the reference speed used for an open request.
	done := Response{
		Agent: "a", Source: "codex", Output: 50, Final: true,
		Start: now.Add(-100 * time.Millisecond), End: now,
	}
	if got := OutputRate(now, []Response{done}, nil); mathAbs(got-50.0/3) > 0.01 {
		t.Fatalf("short = %v", got)
	}
	past := done
	past.End = now.Add(-time.Minute)
	past.Start = past.End.Add(-100 * time.Millisecond)
	flight := Flight{Agent: "a", Source: "codex", Start: now.Add(-3 * time.Second), Last: now}
	// 50 tokens / 500 ms = 100 tok/s, not 50/100ms. The open request fills the window.
	if got := OutputRate(now, []Response{past}, []Flight{flight}); mathAbs(got-100) > 0.01 {
		t.Fatalf("widened estimate = %v", got)
	}
}

func TestPartsShowsEachRunningSourceSideBySide(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	window := -3 * time.Second
	grok := Response{Agent: "g", Source: "grok", Output: 150, Final: true, Start: now.Add(window), End: now}
	codex := Response{Agent: "c", Source: "codex", Output: 300, Final: true, Start: now.Add(window), End: now}
	old := Response{Agent: "k", Source: "claude", Output: 100000, Final: true,
		Start: now.Add(-10 * time.Minute), End: now.Add(-10*time.Minute + 3*time.Second)}
	got := Parts(now, []Response{grok, old, codex}, nil)
	if len(got) != 2 || got[0].Source != "codex" || got[0].Text != "100 tok/s" || got[1].Source != "grok" || got[1].Text != "50 tok/s" {
		t.Fatalf("parts = %+v", got)
	}
}

func TestFormatDropsARateThatRoundsToZero(t *testing.T) {
	if text, ok := Format(0.04); ok || text != "" {
		t.Fatalf("tiny = %q %v", text, ok)
	}
	text, ok := Format(42)
	if !ok || text != "42 tok/s" {
		t.Fatalf("whole = %q %v", text, ok)
	}
	text, ok = Format(42.44)
	if !ok || text != "42.4 tok/s" {
		t.Fatalf("tenth = %q %v", text, ok)
	}
}

func TestPartsGrokPendingYieldsToMeasuredSpeed(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	flight := Flight{Agent: "new", Source: "grok", Start: now.Add(-time.Minute), Last: now}
	parts := Parts(now, nil, []Flight{flight})
	if len(parts) != 1 || !parts[0].Pending || parts[0].Text != "" {
		t.Fatalf("first turn = %+v", parts)
	}
	measured := Response{Agent: "other", Source: "grok", Output: 100, Final: true,
		Start: now.Add(-2 * time.Minute), End: now.Add(-2*time.Minute + 2*time.Second)}
	parts = Parts(now, []Response{measured}, []Flight{flight})
	if len(parts) != 1 || parts[0].Pending || parts[0].Text != "50 tok/s" {
		t.Fatalf("measured speed = %+v", parts)
	}
	for _, source := range []string{"codex", "claude"} {
		flight.Source = source
		if got := Parts(now, nil, []Flight{flight}); len(got) != 0 {
			t.Fatalf("changed %s behavior: %+v", source, got)
		}
	}
	flight.Source = "grok"
	for _, invalid := range []Flight{
		{Source: "grok", Last: now},
		{Source: "grok", Start: now.Add(-time.Minute)},
		{Source: "grok", Start: now.Add(time.Second), Last: now},
		{Source: "grok", Start: flight.Start, Last: now.Add(time.Second)},
		{Source: "grok", Start: flight.Start, Last: now.Add(-flightFresh - time.Second)},
	} {
		if got := Parts(now, nil, []Flight{invalid}); len(got) != 0 {
			t.Fatalf("invalid flight produced %+v", got)
		}
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
