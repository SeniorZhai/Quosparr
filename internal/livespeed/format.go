package livespeed

import (
	"math"
	"strconv"
	"time"
)

// Format rounds a positive rate to a tenth and renders it as tok/s.
// A rate that rounds to zero is not shown.
func Format(rate float64) (string, bool) {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
		return "", false
	}
	tenths := int64(math.Round(rate * 10))
	if tenths < 1 {
		return "", false
	}
	whole, frac := tenths/10, tenths%10
	text := strconv.FormatInt(whole, 10)
	if frac != 0 {
		text += "." + strconv.FormatInt(frac, 10)
	}
	return text + " tok/s", true
}

// Part is one source with a rate or a wait for completed usage.
type Part struct {
	Source  string
	Text    string
	Pending bool
	Last    bool
}

// Parts is the live rate of each source that is producing tokens, in a
// stable order. Grok only reports completed turns, so its rate is labeled Last.
func Parts(now time.Time, responses []Response, flights []Flight) []Part {
	var out []Part
	for _, source := range []string{"codex", "claude", "grok"} {
		active := flightsBySource(flights, source)
		if source == "grok" {
			pending := false
			for _, f := range active {
				if !f.Start.IsZero() && f.Start.Before(now) && !f.Last.IsZero() && !f.Last.After(now) && now.Sub(f.Last) <= flightFresh {
					pending = true
					break
				}
			}
			r := latestGrok(now, responses)
			if !r.End.IsZero() && (pending || now.Sub(r.End) <= 30*time.Second) {
				if text, ok := Format(float64(r.Output) / r.End.Sub(r.Start).Seconds()); ok {
					out = append(out, Part{Source: source, Text: text, Last: true})
					continue
				}
			}
			if pending {
				out = append(out, Part{Source: source, Pending: true})
			}
			continue
		}
		if text, on := Format(OutputRate(now, bySource(responses, source), active)); on {
			out = append(out, Part{Source: source, Text: text})
		}
	}
	return out
}

const grokHistoryWindow = 24 * time.Hour

func latestGrok(now time.Time, responses []Response) Response {
	var latest Response
	for _, r := range responses {
		if r.Source == "grok" && r.Final && r.Output > 0 && !r.Start.IsZero() && r.End.After(r.Start) &&
			r.End.Sub(r.Start) <= grokHistoryWindow && !r.End.After(now) && now.Sub(r.End) <= grokHistoryWindow && r.End.After(latest.End) {
			latest = r
		}
	}
	return latest
}

func bySource(in []Response, source string) []Response {
	var out []Response
	for _, r := range in {
		if r.Source == source {
			out = append(out, r)
		}
	}
	return out
}

func flightsBySource(in []Flight, source string) []Flight {
	var out []Flight
	for _, f := range in {
		if f.Source == source {
			out = append(out, f)
		}
	}
	return out
}
