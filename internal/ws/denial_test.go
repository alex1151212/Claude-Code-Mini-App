package ws

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/agent"
)

func bashDenial(id, cmd string, blocked bool) agent.PermissionDenial {
	in, _ := json.Marshal(map[string]string{"command": cmd})
	return agent.PermissionDenial{ToolName: "Bash", ToolUseID: id, ToolInput: in, BlockedByRule: blocked}
}

func TestSplitDenials(t *testing.T) {
	t.Parallel()
	pending, blocked := splitDenials([]agent.PermissionDenial{
		bashDenial("a", "rm -rf x", true),
		bashDenial("b", "go build", false),
	})
	if len(pending) != 1 || pending[0].ToolUseID != "b" {
		t.Fatalf("pending=%+v", pending)
	}
	if len(blocked) != 1 || blocked[0].ToolUseID != "a" {
		t.Fatalf("blocked=%+v", blocked)
	}
}

func TestSplitStillDenied(t *testing.T) {
	t.Parallel()
	ds := []agent.PermissionDenial{
		bashDenial("a", "rm -rf x", false),
		{ToolName: "Write", ToolUseID: "w", ToolInput: json.RawMessage(`{"file_path":"a.txt"}`)},
	}
	// 重跑只放行 Bash：Bash 又被拒 = 規則壓過 allow；Write 是重跑途中新出現的授權需求，維持待授權。
	pending, still := splitStillDenied([]string{"Bash"}, ds)
	if len(still) != 1 || still[0].ToolUseID != "a" {
		t.Fatalf("still=%+v", still)
	}
	if len(pending) != 1 || pending[0].ToolUseID != "w" {
		t.Fatalf("pending=%+v", pending)
	}
	// 非重跑（allowed 為空）：全部維持待授權。
	pending, still = splitStillDenied(nil, ds)
	if len(pending) != 2 || len(still) != 0 {
		t.Fatalf("非重跑不該有 still: pending=%d still=%d", len(pending), len(still))
	}
	// 帶 specifier 的 allowed 也以工具名比對。
	if _, still = splitStillDenied([]string{"Bash(rm -rf x)"}, ds); len(still) != 1 {
		t.Fatalf("specifier 形式應比工具名: still=%+v", still)
	}
}

func TestDenialNotices(t *testing.T) {
	t.Parallel()
	d := bashDenial("a", "rm -rf .demo\ncmd/zz_seed `x`", true)
	n := blockedNotice([]agent.PermissionDenial{d})
	for _, want := range []string{"permissions.deny", "按「允許」也無效", "rm -rf .demo cmd/zz_seed 'x'", "settings.json"} {
		if !strings.Contains(n, want) {
			t.Errorf("blockedNotice 缺少 %q:\n%s", want, n)
		}
	}
	s := stillDeniedNotice([]agent.PermissionDenial{d})
	if !strings.Contains(s, "已允許但") || !strings.Contains(s, "permissions.ask") {
		t.Errorf("stillDeniedNotice:\n%s", s)
	}
	if blockedNotice(nil) != "" || stillDeniedNotice(nil) != "" {
		t.Error("空清單不應產生說明")
	}
}

func TestDenialTarget(t *testing.T) {
	t.Parallel()
	if got := denialTarget(agent.PermissionDenial{ToolName: "Edit", ToolInput: json.RawMessage(`{"file_path":"/a/b.go"}`)}); got != "/a/b.go" {
		t.Errorf("file_path: %q", got)
	}
	if got := denialTarget(agent.PermissionDenial{ToolName: "Task", ToolInput: json.RawMessage(`{"x":1}`)}); got != "Task" {
		t.Errorf("無已知欄位應回工具名: %q", got)
	}
	long := strings.Repeat("長", 300)
	got := denialTarget(bashDenial("a", long, false))
	if r := []rune(got); len(r) != 161 {
		t.Errorf("應截到 160 字 + …，got %d", len(r))
	}
}
