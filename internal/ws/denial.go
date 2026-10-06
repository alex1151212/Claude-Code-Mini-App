package ws

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/agent"
)

// splitDenials 把 Claude 回報的 denial 分成「可等使用者授權」與「被 settings deny 規則擋下」兩類。
// 後者按「允許」重跑也不會通過，不能進等授權。
func splitDenials(ds []agent.PermissionDenial) (pending, blocked []agent.PermissionDenial) {
	for _, d := range ds {
		if d.BlockedByRule {
			blocked = append(blocked, d)
		} else {
			pending = append(pending, d)
		}
	}
	return pending, blocked
}

// splitStillDenied 在「允許此操作」的重跑中，把仍被拒的 denial 挑出來：
// 工具已在本次 --allowedTools 內卻還是被拒，表示有 ask／deny 規則或政策壓過 allow，再等授權只會無限迴圈。
// 其餘（例如重跑途中 agent 換用另一個需授權的工具）維持待授權。
func splitStillDenied(allowed []string, ds []agent.PermissionDenial) (pending, still []agent.PermissionDenial) {
	for _, d := range ds {
		if coveredByAllowed(allowed, d) {
			still = append(still, d)
		} else {
			pending = append(pending, d)
		}
	}
	return pending, still
}

// coveredByAllowed：allowed 的元素是工具名（Bash）或帶 specifier 的規則（Bash(git *)）；只比工具名，
// 刻意寬鬆——帶 specifier 的規則沒命中也算「仍被拒」，寧可提示設定問題，也不要再讓使用者按一次沒用的允許。
func coveredByAllowed(allowed []string, d agent.PermissionDenial) bool {
	for _, a := range allowed {
		name := strings.TrimSpace(a)
		if i := strings.Index(name, "("); i >= 0 {
			name = name[:i]
		}
		if name != "" && name == d.ToolName {
			return true
		}
	}
	return false
}

// denialTarget 取出讓人看得懂的被拒對象（Bash 指令、檔案路徑、URL），過長截斷；取不到就回工具名。
func denialTarget(d agent.PermissionDenial) string {
	var in map[string]any
	if len(d.ToolInput) > 0 && json.Unmarshal(d.ToolInput, &in) == nil {
		for _, k := range []string{"command", "file_path", "path", "url"} {
			if s, ok := in[k].(string); ok && strings.TrimSpace(s) != "" {
				return truncateRunes(strings.TrimSpace(s), 160)
			}
		}
	}
	return d.ToolName
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func denialList(ds []agent.PermissionDenial) string {
	var sb strings.Builder
	for _, d := range ds {
		// 換行壓成空格，避免多行指令打亂引用區塊。
		t := strings.Join(strings.Fields(denialTarget(d)), " ")
		fmt.Fprintf(&sb, "> - `%s`：`%s`\n", d.ToolName, strings.ReplaceAll(t, "`", "'"))
	}
	return sb.String()
}

const settingsHint = "請檢查 `~/.claude/settings.json`、專案 `.claude/settings.json`／`settings.local.json` 的 `permissions`。"

// blockedNotice：指令被 permissions.deny 規則擋下時，附在回覆後的說明。
func blockedNotice(ds []agent.PermissionDenial) string {
	if len(ds) == 0 {
		return ""
	}
	return "\n\n> ⚠️ 以下操作被 Claude 權限規則（`permissions.deny`）禁止，按「允許」也無效：\n" +
		denialList(ds) + "> " + settingsHint + "\n"
}

// stillDeniedNotice：已按允許重跑，仍被拒時的說明。
func stillDeniedNotice(ds []agent.PermissionDenial) string {
	if len(ds) == 0 {
		return ""
	}
	return "\n\n> ⚠️ 已允許但以下操作仍被拒絕，通常是 settings 內的 `permissions.ask`／`deny` 規則所致：\n" +
		denialList(ds) + "> " + settingsHint + "\n"
}
