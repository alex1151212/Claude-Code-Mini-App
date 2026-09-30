package quota

import (
	"testing"
	"time"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/usage"
)

func TestFormatClaudeDisplay(t *testing.T) {
	now := time.Date(2026, 6, 30, 11, 45, 0, 0, time.FixedZone("CST", 8*3600))
	info := usage.FromClaudeUsageText(`Current session: 16% used · resets Jun 30, 3:10pm (Asia/Taipei)
Current week (all models): 9% used · resets Jul 3, 9:59pm (Asia/Taipei)`)
	got := FormatDisplayAt("claude", info, now)
	if got != "5h 16% · Resets in 3 hr 25 min · Week 9%" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatClaudeDisplayOAuth(t *testing.T) {
	now := time.Date(2026, 6, 30, 11, 45, 0, 0, time.FixedZone("CST", 8*3600))
	info, err := usage.FromClaudeOAuthUsageJSON([]byte(`{"five_hour":{"utilization":16,"resets_at":"2026-06-30T15:09:59+08:00"},"seven_day":{"utilization":9,"resets_at":"2026-07-03T22:00:00+08:00"}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := FormatDisplayAt("claude", info, now)
	if got != "5h 16% · Resets in 3 hr 25 min · Week 9%" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatCursorDisplay(t *testing.T) {
	raw := `{"planUsage":{"autoPercentUsed":8.05,"apiPercentUsed":100,"totalPercentUsed":30.28}}`
	info, err := usage.FromCursorPeriodUsageJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	got := FormatDisplay("cursor", info)
	if got != "Total 30% · Auto 8% · API 100%" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatKiroDisplay(t *testing.T) {
	info := usage.FromKiroUsageText("KIRO FREE\nCredits (4.52 of 50)\n9%\n")
	got := FormatDisplay("kiro", info)
	if got != "FREE · Credits 9% (4.52/50)" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatCodexDisplay(t *testing.T) {
	now := time.Unix(1790700000, 0)
	info := usage.FromCodexRolloutLine([]byte(sampleCodexRolloutLine), now)
	got := FormatDisplay("codex", info)
	if got != "5h 2% · Week 13%" {
		t.Fatalf("got %q", got)
	}
}

// codex-cli 0.159.2 實測 rollout token_count 行（週用量改為 13 以驗證非零）。
const sampleCodexRolloutLine = `{"timestamp":"2026-09-30T06:44:51.387Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":38134}},"rate_limits":{"limit_id":"codex","primary":{"used_percent":2.0,"window_minutes":300,"resets_at":1790767809},"secondary":{"used_percent":13,"window_minutes":10080,"resets_at":1791354609},"credits":{"has_credits":false},"plan_type":"plus"}}}`

func TestServiceGetAntigravity(t *testing.T) {
	s := NewService()
	snap := s.Get("antigravity")
	if snap.DisplayText != "未實作" {
		t.Fatalf("got %q", snap.DisplayText)
	}
	// 舊 agent_type 別名
	snap = s.Get("gemini")
	if snap.DisplayText != "未實作" {
		t.Fatalf("gemini alias got %q", snap.DisplayText)
	}
}
