// Package livespeed is the live output rate of local coding agents.
// A number here is the last three seconds, never a historical average.
package livespeed

import (
	"math"
	"time"
)

const (
	window         = 3 * time.Second
	minDuration    = 500 * time.Millisecond
	maxDuration    = 15 * time.Minute
	recentWindow   = 10 * time.Minute
	fallbackWindow = 30 * time.Minute
	flightFresh    = 5 * time.Minute
	recentLimit    = 3
	fallbackLimit  = 10
)

// Response is one model call read from a local log.
// Start is zero when the request time is unknown.
// Final is false for a streaming snapshot that is not the completed usage.
type Response struct {
	Agent, Model, Source string
	Output               int64
	Start, End           time.Time
	Final                bool
}

// Flight is a request still being answered.
type Flight struct {
	Agent, Model, Source string
	Start, Last          time.Time
}

// OutputRate is output tokens per second over the three seconds ending at now.
// Zero means there is nothing to show.
func OutputRate(now time.Time, responses []Response, flights []Flight) float64 {
	if now.IsZero() {
		return 0
	}
	keep := responses[:0:0]
	cutoff := now.Add(-fallbackWindow)
	for _, r := range responses {
		if r.End.Before(cutoff) || r.Output < 0 {
			continue
		}
		if r.Output > 1e12 {
			r.Output = 1e12
		}
		keep = append(keep, r)
	}
	from := now.Add(-window)
	var tokens float64
	for _, r := range keep {
		if !valid(r) {
			continue
		}
		start, dur := widen(r)
		share := overlap(from, now, start, r.End)
		if share <= 0 {
			continue
		}
		if r.Final {
			tokens += float64(r.Output) * share.Seconds() / dur.Seconds()
			continue
		}
		speed, ok := estimate(now, keep, r.Agent, r.Model, r.Source)
		if ok {
			tokens += speed * share.Seconds()
		}
	}
	for _, f := range flights {
		if f.Start.IsZero() || f.Last.IsZero() || now.Sub(f.Last) > flightFresh {
			continue
		}
		speed, ok := estimate(now, keep, f.Agent, f.Model, f.Source)
		if !ok {
			continue
		}
		begin := f.Start
		if end := latestEnd(keep, f.Agent); end.After(begin) {
			begin = end
		}
		tokens += speed * overlap(from, now, begin, now).Seconds()
	}
	if tokens <= 0 || math.IsNaN(tokens) || math.IsInf(tokens, 0) {
		return 0
	}
	return tokens / window.Seconds()
}

func valid(r Response) bool {
	return !r.Start.IsZero() && !r.End.Before(r.Start) && r.End.Sub(r.Start) <= maxDuration
}

func widen(r Response) (time.Time, time.Duration) {
	dur := r.End.Sub(r.Start)
	if dur < minDuration {
		return r.End.Add(-minDuration), minDuration
	}
	return r.Start, dur
}

func overlap(a0, a1, b0, b1 time.Time) time.Duration {
	if b0.After(a0) {
		a0 = b0
	}
	if b1.Before(a1) {
		a1 = b1
	}
	if !a1.After(a0) {
		return 0
	}
	return a1.Sub(a0)
}

func latestEnd(responses []Response, agent string) time.Time {
	var end time.Time
	for _, r := range responses {
		if r.Agent == agent && r.End.After(end) {
			end = r.End
		}
	}
	return end
}

func estimate(now time.Time, responses []Response, agent, model, source string) (float64, bool) {
	if v, ok := reference(now, responses, recentLimit, recentWindow, func(r Response) bool { return r.Agent == agent }); ok {
		return v, true
	}
	if model != "" {
		if v, ok := reference(now, responses, fallbackLimit, fallbackWindow, func(r Response) bool { return r.Model == model }); ok {
			return v, true
		}
	}
	return reference(now, responses, fallbackLimit, fallbackWindow, func(r Response) bool { return r.Source == source })
}

func reference(now time.Time, responses []Response, limit int, window time.Duration, match func(Response) bool) (float64, bool) {
	cutoff := now.Add(-window)
	picked := make([]Response, 0, limit)
	for _, r := range responses {
		if !r.Final || !valid(r) || r.End.Before(cutoff) || !match(r) {
			continue
		}
		picked = insertRecent(picked, r, limit)
	}
	var output, secs float64
	for _, r := range picked {
		_, dur := widen(r)
		output += float64(r.Output)
		secs += dur.Seconds()
	}
	if secs <= 0 {
		return 0, false
	}
	return output / secs, true
}

// insertRecent keeps the newest responses, capped at limit.
func insertRecent(picked []Response, r Response, limit int) []Response {
	i := 0
	for i < len(picked) && !r.End.After(picked[i].End) {
		i++
	}
	if i == limit {
		return picked
	}
	picked = append(picked, Response{})
	copy(picked[i+1:], picked[i:])
	picked[i] = r
	if len(picked) > limit {
		picked = picked[:limit]
	}
	return picked
}
