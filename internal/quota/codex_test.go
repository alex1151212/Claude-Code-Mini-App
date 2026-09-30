package quota

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeRollout(t *testing.T, path, content string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func rl(pct string) string {
	return `{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":` + pct + `,"window_minutes":300,"resets_at":1790767809},"secondary":{"used_percent":1,"window_minutes":10080,"resets_at":1791354609}}}}` + "\n"
}

func TestLatestCodexRateLimits(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1790700000, 0)
	base := now.Add(-time.Hour)

	// 今天目錄裡較舊的檔。
	writeRollout(t, filepath.Join(root, "2026", "09", "30", "rollout-a.jsonl"), rl("5"), base)
	// 三天前建立、今天被 resume 的 thread：mtime 最新但位在舊日期目錄，應優先。
	// 最後一筆 rate_limits 才算數；尾端不完整的行（寫入中）要被略過。
	writeRollout(t, filepath.Join(root, "2026", "09", "27", "rollout-b.jsonl"),
		rl("7")+rl("9")+`{"type":"event_msg","payload":{"rate_limits":{"prim`, base.Add(30*time.Minute))
	// 最新但失敗的回合（無 rate_limits）：應被跳過。
	writeRollout(t, filepath.Join(root, "2026", "09", "30", "rollout-c.jsonl"),
		`{"type":"session_meta","payload":{}}`+"\n", base.Add(50*time.Minute))

	got := FormatDisplay("codex", latestCodexRateLimits(root, now))
	if got != "5h 9% · Week 1%" {
		t.Fatalf("got %q", got)
	}

	if latestCodexRateLimits(filepath.Join(root, "missing"), now) != nil {
		t.Fatal("missing dir should be nil")
	}
}
