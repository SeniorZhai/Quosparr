package usage

import (
	"context"
	"fmt"
)

// DailyTokenReport counts only usage records; absent days have no recorded data.
type DailyTokenReport struct {
	Days        map[string]int64
	UnreadLines int
}

// ScanDaily reuses the read-only, deduplicated in-memory log cache.
func (s Scanner) ScanDaily(ctx context.Context, window Window) (DailyTokenReport, error) {
	if !window.End.After(window.Start) {
		return DailyTokenReport{}, fmt.Errorf("usage: invalid daily-token window")
	}
	sources, err := s.cacheSources(ctx)
	if err != nil {
		return DailyTokenReport{}, err
	}
	out := DailyTokenReport{Days: map[string]int64{}}
	for _, source := range sources {
		if err := ctx.Err(); err != nil {
			return DailyTokenReport{}, err
		}
		file, err := s.cacheSamples(ctx, source)
		if err != nil {
			return DailyTokenReport{}, err
		}
		if file.meta.MtimeNS >= window.Start.UnixNano() {
			out.UnreadLines += file.unread
		}
		for _, sample := range file.samples {
			if err := ctx.Err(); err != nil {
				return DailyTokenReport{}, err
			}
			if !window.Contains(sample.at) {
				continue
			}
			tokens, err := checkedAdd(sample.input, sample.output, "daily tokens")
			if err != nil {
				return DailyTokenReport{}, err
			}
			day := dayKey(sample.at, s.loc())
			out.Days[day], err = checkedAdd(out.Days[day], tokens, "daily tokens")
			if err != nil {
				return DailyTokenReport{}, err
			}
		}
	}
	return out, nil
}
