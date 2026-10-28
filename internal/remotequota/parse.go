package remotequota

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

// Reading codes. Empty means a usable reading, which may still have no meters.
const (
	CodeUnconfigured = "unconfigured"
	CodeUnauthorized = "unauthorized"
	CodeUnavailable  = "unavailable"
	CodeFailed       = "failed"
)

// Pool IDs.
const (
	PoolSession   = "5h"
	PoolWeek      = "7d"
	PoolMonth     = "month"
	PoolPremium   = "premium"
	PoolChat      = "chat"
	PoolBalance   = "balance"
	PoolCredits   = "credits"
	LineGranted   = "granted"
	LineTopped    = "topped"
	LineMonthCode = "month_code"
	LineDeficit   = "deficit"
)

// Pool is one quota window or a balance amount.
type Pool struct {
	ID            string
	UsedPercent   int
	WindowMinutes int
	ResetsAt      time.Time
	Balance       bool
	ValueText     string
}

// Line is a non-secret detail row. ID is a stable name, not a sentence.
type Line struct {
	ID    string
	Value string
}

// Reading is a parsed quota response.
type Reading struct {
	Code  string
	Plan  string
	Pools []Pool
	Lines []Line
}

// ParseKimi reads a Kimi Code usages response.
func ParseKimi(body []byte) (Reading, error) {
	var raw struct {
		Usages struct {
			Session     *ratioPool `json:"limit_5h"`
			Weekly      *ratioPool `json:"limit_7d"`
			Monthly     *ratioPool `json:"limit_month_total"`
			MonthlyCode *ratioPool `json:"limit_month_code"`
		} `json:"usages"`
		User struct {
			Membership struct {
				Level string `json:"level"`
			} `json:"membership"`
		} `json:"user"`
		Version any `json:"version"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Reading{Code: CodeFailed}, err
	}
	out := Reading{Plan: kimiPlan(raw.User.Membership.Level, raw.Version)}
	add := func(id string, minutes int, pool *ratioPool) {
		if pool == nil || pool.UsedRatio == nil || math.IsNaN(*pool.UsedRatio) || *pool.UsedRatio < 0 {
			return
		}
		used := int(math.Round(math.Min(1, *pool.UsedRatio) * 100))
		out.Pools = append(out.Pools, Pool{
			ID: id, UsedPercent: used, WindowMinutes: minutes, ResetsAt: parseTime(pool.ResetTime),
		})
	}
	add(PoolSession, 300, raw.Usages.Session)
	add(PoolWeek, 10080, raw.Usages.Weekly)
	add(PoolMonth, 30*24*60, raw.Usages.Monthly)
	if raw.Usages.MonthlyCode != nil && raw.Usages.MonthlyCode.UsedRatio != nil && *raw.Usages.MonthlyCode.UsedRatio >= 0 {
		used := int(math.Round(math.Min(1, *raw.Usages.MonthlyCode.UsedRatio) * 100))
		out.Lines = append(out.Lines, Line{ID: LineMonthCode, Value: strconv.Itoa(used) + "%"})
	}
	if len(out.Pools) == 0 {
		out.Code = CodeFailed
		return out, nil
	}
	return out, nil
}

// ParseMoonshot reads an Open Platform balance. region selects the currency:
// China is CNY and anything else international is USD.
func ParseMoonshot(body []byte, region string) (Reading, error) {
	var raw struct {
		Code   *int `json:"code"`
		Status bool `json:"status"`
		Data   *struct {
			Available float64 `json:"available_balance"`
			Cash      float64 `json:"cash_balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.Data == nil {
		return Reading{Code: CodeFailed}, err
	}
	if raw.Code != nil && *raw.Code != 0 {
		return Reading{Code: CodeFailed}, nil
	}
	symbol := "¥"
	if normalizeRegion(region) == RegionIntl {
		symbol = "$"
	}
	used := 0
	code := ""
	if !raw.Status || raw.Data.Available <= 0 {
		used = 100
		code = CodeUnavailable
	}
	out := Reading{Code: code, Pools: []Pool{{
		ID: PoolBalance, Balance: true, UsedPercent: used, ValueText: symbol + formatMoney(raw.Data.Available),
	}}}
	if raw.Data.Cash < 0 {
		out.Lines = append(out.Lines, Line{ID: LineDeficit, Value: symbol + formatMoney(raw.Data.Cash)})
	}
	return out, nil
}

func kimiPlan(level string, version any) string {
	level = strings.TrimSpace(level)
	if level == "" || level == "LEVEL_UNSPECIFIED" {
		return ""
	}
	switch version.(type) {
	case nil:
	case string:
		if version.(string) != "" && version.(string) != "GOODS_VERSION_V1" {
			return level
		}
	default:
		return level
	}
	switch level {
	case "LEVEL_FREE":
		return "Adagio"
	case "LEVEL_TRIAL":
		return "Andante"
	case "LEVEL_BASIC":
		return "Moderato"
	case "LEVEL_INTERMEDIATE":
		return "Allegretto"
	case "LEVEL_ADVANCED":
		return "Allegro"
	default:
		return level
	}
}

type ratioPool struct {
	UsedRatio *float64 `json:"used_ratio"`
	ResetTime string   `json:"reset_time"`
}

// ParseDeepSeek reads a DeepSeek balance response.
func ParseDeepSeek(body []byte) (Reading, error) {
	var raw struct {
		Available bool `json:"is_available"`
		Infos     []struct {
			Currency string `json:"currency"`
			Total    string `json:"total_balance"`
			Granted  string `json:"granted_balance"`
			Topped   string `json:"topped_up_balance"`
		} `json:"balance_infos"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Reading{Code: CodeFailed}, err
	}
	if len(raw.Infos) == 0 {
		return Reading{Code: CodeUnavailable, Pools: []Pool{{
			ID: PoolBalance, Balance: true, UsedPercent: 100, ValueText: "0",
		}}}, nil
	}
	info := raw.Infos[0]
	total, err := strconv.ParseFloat(strings.TrimSpace(info.Total), 64)
	if err != nil {
		return Reading{Code: CodeFailed}, nil
	}
	symbol := "$"
	if strings.EqualFold(info.Currency, "CNY") {
		symbol = "¥"
	}
	text := symbol + formatMoney(total)
	used := 0
	code := ""
	if total <= 0 || !raw.Available {
		used = 100
		code = CodeUnavailable
	}
	out := Reading{Code: code, Pools: []Pool{{
		ID: PoolBalance, Balance: true, UsedPercent: used, ValueText: text,
	}}}
	if granted, err := strconv.ParseFloat(strings.TrimSpace(info.Granted), 64); err == nil {
		out.Lines = append(out.Lines, Line{ID: LineGranted, Value: symbol + formatMoney(granted)})
	}
	if topped, err := strconv.ParseFloat(strings.TrimSpace(info.Topped), 64); err == nil {
		out.Lines = append(out.Lines, Line{ID: LineTopped, Value: symbol + formatMoney(topped)})
	}
	return out, nil
}

func formatMoney(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// ParseCopilot reads a GitHub Copilot user payload.
func ParseCopilot(body []byte) (Reading, error) {
	var raw struct {
		Plan         string `json:"copilot_plan"`
		Reset        string `json:"quota_reset_date"`
		TokenBilling bool   `json:"token_based_billing"`
		Snapshots    struct {
			Premium *copilotSnap `json:"premium_interactions"`
			Chat    *copilotSnap `json:"chat"`
		} `json:"quota_snapshots"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Reading{Code: CodeFailed}, err
	}
	resets := parseTime(raw.Reset)
	out := Reading{Plan: strings.TrimSpace(raw.Plan)}
	premium, premiumUnlimited := copilotWindow(raw.Snapshots.Premium)
	chat, chatUnlimited := copilotWindow(raw.Snapshots.Chat)
	if premium != nil {
		premium.ID = PoolPremium
		premium.ResetsAt = resets
		out.Pools = append(out.Pools, *premium)
	}
	if chat != nil {
		chat.ID = PoolChat
		chat.ResetsAt = resets
		out.Pools = append(out.Pools, *chat)
	}
	if len(out.Pools) == 0 {
		if text, ok := copilotCredits(raw.Snapshots.Premium, raw.Snapshots.Chat); ok {
			out.Pools = append(out.Pools, Pool{ID: PoolCredits, Balance: true, ValueText: text})
		} else if !raw.TokenBilling && !premiumUnlimited && !chatUnlimited {
			out.Code = CodeFailed
		}
	}
	return out, nil
}

func copilotCredits(snaps ...*copilotSnap) (string, bool) {
	for _, snap := range snaps {
		if snap == nil || snap.CreditsUsed == nil || math.IsNaN(*snap.CreditsUsed) || math.IsInf(*snap.CreditsUsed, 0) {
			continue
		}
		value := math.Max(0, *snap.CreditsUsed)
		if math.Trunc(value) == value && math.Abs(value) < 1e15 {
			return strconv.FormatInt(int64(value), 10), true
		}
		return strconv.FormatFloat(value, 'f', 2, 64), true
	}
	return "", false
}

type copilotSnap struct {
	Entitlement      *float64 `json:"entitlement"`
	Remaining        *float64 `json:"remaining"`
	PercentRemaining *float64 `json:"percent_remaining"`
	Unlimited        bool     `json:"unlimited"`
	CreditsUsed      *float64 `json:"credits_used"`
}

func copilotWindow(snap *copilotSnap) (*Pool, bool) {
	if snap == nil || snap.Unlimited {
		return nil, snap != nil && snap.Unlimited
	}
	entitled := snap.Entitlement != nil
	remained := snap.Remaining != nil
	if entitled && remained && *snap.Entitlement == 0 && *snap.Remaining == 0 {
		return nil, false
	}
	var percent *float64
	if snap.PercentRemaining != nil {
		percent = snap.PercentRemaining
	} else if entitled && remained && *snap.Entitlement > 0 {
		derived := (*snap.Remaining / *snap.Entitlement) * 100
		percent = &derived
	}
	if percent == nil || math.IsNaN(*percent) {
		return nil, false
	}
	used := int(math.Round(math.Max(0, 100-*percent)))
	return &Pool{UsedPercent: used}, false
}

func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}
