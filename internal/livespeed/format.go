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

// Part is one source that has a live rate.
type Part struct {
	Source  string
	Text    string
	Pending bool
}

// Parts is the live rate of each source that is producing tokens, in a
// stable order. Grok's first turn stays pending until usage is available.
func Parts(now time.Time, responses []Response, flights []Flight) []Part {
	var out []Part
	for _, source := range []string{"codex", "claude", "grok"} {
		active := flightsBySource(flights, source)
		text, on := Format(OutputRate(now, bySource(responses, source), active))
		if on {
			out = append(out, Part{Source: source, Text: text})
		} else if source == "grok" {
			for _, f := range active {
				if !f.Start.IsZero() && f.Start.Before(now) && !f.Last.IsZero() && !f.Last.After(now) && now.Sub(f.Last) <= flightFresh {
					out = append(out, Part{Source: source, Pending: true})
					break
				}
			}
		}
	}
	return out
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
