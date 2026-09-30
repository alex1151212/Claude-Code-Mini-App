package usage

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// QuotaWindow 表示單一用量窗口（5 小時、週、帳單週期等）。
type QuotaWindow struct {
	Kind     string   `json:"kind"`               // session | weekly | billing | credits
	Label    string   `json:"label,omitempty"`
	Percent  *float64 `json:"percent,omitempty"`
	Used     *float64 `json:"used,omitempty"`
	Limit    *float64 `json:"limit,omitempty"`
	ResetsAt string   `json:"resets_at,omitempty"`
}

// QuotaInfo 是跨 provider 正規化後的帳戶用量百分比。
type QuotaInfo struct {
	Provider string        `json:"provider"`
	Plan     string        `json:"plan,omitempty"`
	Windows  []QuotaWindow `json:"windows"`
	Source   string        `json:"source"` // 資料來源描述
}

var (
	claudeSessionRe = regexp.MustCompile(`(?i)Current session:\s*(\d+(?:\.\d+)?)%\s*used\s+[^\r\n]*?resets\s*(.+)`)
	claudeWeeklyRe  = regexp.MustCompile(`(?i)Current week \(all models\):\s*(\d+(?:\.\d+)?)%\s*used\s+[^\r\n]*?resets\s*(.+)`)
	kiroQuotaCreditsRe = regexp.MustCompile(`Credits\s*\(([\d.]+)\s+of\s+([\d.]+)`)
	kiroQuotaPercentRe = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
)

// FromClaudeUsageText 解析 `claude -p "/usage"` 的純文字輸出。
func FromClaudeUsageText(text string) *QuotaInfo {
	out := &QuotaInfo{Provider: "claude", Source: "claude -p /usage text"}
	if m := claudeSessionRe.FindStringSubmatch(text); len(m) == 3 {
		out.Windows = append(out.Windows, QuotaWindow{
			Kind:     "session",
			Label:    "Current session",
			Percent:  floatPtr(parseFloat(m[1])),
			ResetsAt: strings.TrimSpace(m[2]),
		})
	}
	if m := claudeWeeklyRe.FindStringSubmatch(text); len(m) == 3 {
		out.Windows = append(out.Windows, QuotaWindow{
			Kind:     "weekly",
			Label:    "Current week (all models)",
			Percent:  floatPtr(parseFloat(m[1])),
			ResetsAt: strings.TrimSpace(m[2]),
		})
	}
	return out
}

type claudeOAuthUsage struct {
	FiveHour *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
	SevenDay *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
	Limits []struct {
		Kind     string  `json:"kind"`
		Percent  float64 `json:"percent"`
		ResetsAt string  `json:"resets_at"`
	} `json:"limits"`
}

// FromClaudeOAuthUsageJSON 解析 Anthropic OAuth usage API 回應。
func FromClaudeOAuthUsageJSON(raw []byte) (*QuotaInfo, error) {
	var body claudeOAuthUsage
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	out := &QuotaInfo{Provider: "claude", Source: "api.anthropic.com/api/oauth/usage"}
	if body.FiveHour != nil {
		out.Windows = append(out.Windows, QuotaWindow{
			Kind: "session", Label: "five_hour",
			Percent:  floatPtr(body.FiveHour.Utilization),
			ResetsAt: body.FiveHour.ResetsAt,
		})
	}
	if body.SevenDay != nil {
		out.Windows = append(out.Windows, QuotaWindow{
			Kind: "weekly", Label: "seven_day",
			Percent:  floatPtr(body.SevenDay.Utilization),
			ResetsAt: body.SevenDay.ResetsAt,
		})
	}
	for _, lim := range body.Limits {
		out.Windows = append(out.Windows, QuotaWindow{
			Kind:     lim.Kind,
			Percent:  floatPtr(lim.Percent),
			ResetsAt: lim.ResetsAt,
		})
	}
	return out, nil
}

type cursorPeriodUsage struct {
	PlanUsage *struct {
		AutoPercentUsed  float64 `json:"autoPercentUsed"`
		APIPercentUsed   float64 `json:"apiPercentUsed"`
		TotalPercentUsed float64 `json:"totalPercentUsed"`
		TotalSpend       int64   `json:"totalSpend"`
		Limit            int64   `json:"limit"`
		Remaining        int64   `json:"remaining"`
	} `json:"planUsage"`
	DisplayMessage string `json:"displayMessage"`
}

// FromCursorPeriodUsageJSON 解析 Cursor GetCurrentPeriodUsage 回應。
func FromCursorPeriodUsageJSON(raw []byte) (*QuotaInfo, error) {
	var body cursorPeriodUsage
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	out := &QuotaInfo{Provider: "cursor", Source: "api2.cursor.sh GetCurrentPeriodUsage"}
	if body.PlanUsage == nil {
		return out, nil
	}
	pu := body.PlanUsage
	out.Windows = append(out.Windows,
		QuotaWindow{Kind: "billing_auto", Label: "autoPercentUsed", Percent: floatPtr(pu.AutoPercentUsed)},
		QuotaWindow{Kind: "billing_api", Label: "apiPercentUsed", Percent: floatPtr(pu.APIPercentUsed)},
		QuotaWindow{Kind: "billing_total", Label: "totalPercentUsed", Percent: floatPtr(pu.TotalPercentUsed)},
	)
	if pu.Limit > 0 {
		used := float64(pu.TotalSpend) / 100.0
		limit := float64(pu.Limit) / 100.0
		out.Windows = append(out.Windows, QuotaWindow{
			Kind: "billing_spend", Label: "plan spend USD",
			Used: floatPtr(used), Limit: floatPtr(limit),
		})
	}
	return out, nil
}

// FromKiroUsageText 解析 `kiro-cli chat "/usage" --no-interactive` 的 stdout。
func FromKiroUsageText(text string) *QuotaInfo {
	clean := ansiRe.ReplaceAllString(text, "")
	out := &QuotaInfo{Provider: "kiro", Source: "kiro-cli chat /usage --no-interactive"}
	if m := kiroQuotaCreditsRe.FindStringSubmatch(clean); len(m) == 3 {
		used := parseFloat(m[1])
		limit := parseFloat(m[2])
		var pct *float64
		if limit > 0 {
			pct = floatPtr(used / limit * 100)
		}
		if pm := kiroQuotaPercentRe.FindStringSubmatch(clean); len(pm) == 2 {
			pct = floatPtr(parseFloat(pm[1]))
		}
		out.Windows = append(out.Windows, QuotaWindow{
			Kind: "credits", Label: "monthly credits",
			Percent: pct, Used: floatPtr(used), Limit: floatPtr(limit),
		})
	}
	if plan := regexp.MustCompile(`KIRO\s+(\w+)`).FindStringSubmatch(clean); len(plan) == 2 {
		out.Plan = plan[1]
	}
	return out
}

type codexRateWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes int     `json:"window_minutes"`
	ResetsAt      int64   `json:"resets_at"` // unix 秒
}

// FromCodexRolloutLine 解析 codex rollout 檔（~/.codex/sessions/**/rollout-*.jsonl）
// 中 token_count 事件的 payload.rate_limits；該行無 rate_limits 時回 nil。
// codex 只在有回合時寫檔，窗口 resets_at 已過代表該窗已重置，視為 0%。
func FromCodexRolloutLine(line []byte, now time.Time) *QuotaInfo {
	var rec struct {
		Payload struct {
			RateLimits *struct {
				Primary   *codexRateWindow `json:"primary"`
				Secondary *codexRateWindow `json:"secondary"`
				PlanType  string           `json:"plan_type"`
			} `json:"rate_limits"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &rec) != nil || rec.Payload.RateLimits == nil {
		return nil
	}
	rl := rec.Payload.RateLimits
	out := &QuotaInfo{Provider: "codex", Plan: rl.PlanType, Source: "codex rollout rate_limits"}
	for _, w := range []*codexRateWindow{rl.Primary, rl.Secondary} {
		if w == nil {
			continue
		}
		// 依窗口長度分類而非 primary/secondary 位置，避免方案調整窗口時對錯。
		kind := "session"
		if w.WindowMinutes >= 7*24*60 {
			kind = "weekly"
		}
		pct := w.UsedPercent
		qw := QuotaWindow{Kind: kind, Label: fmt.Sprintf("%d min", w.WindowMinutes)}
		if w.ResetsAt > 0 {
			reset := time.Unix(w.ResetsAt, 0)
			if !now.Before(reset) {
				pct = 0
			}
			qw.ResetsAt = reset.Format(time.RFC3339)
		}
		qw.Percent = floatPtr(pct)
		out.Windows = append(out.Windows, qw)
	}
	if len(out.Windows) == 0 {
		return nil
	}
	return out
}

func floatPtr(f float64) *float64 { return &f }
