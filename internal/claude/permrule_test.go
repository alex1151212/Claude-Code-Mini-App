package claude

import (
	"slices"
	"testing"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/agent"
)

// 實測 claude 2.1.286 被 permissions.deny 擋下時的 user 事件與 result 事件（節錄關鍵欄位）。
const (
	userRuleBlockedLine = `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"Permission to use Bash with command rm -rf x1 has been denied.","is_error":true,"tool_use_id":"toolu_A"}]},"tool_result_meta":[{"id":"toolu_A","non_execution_kind":"permission-rule"}]}`
	userRejectedLine    = `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"Claude requested permissions to use Bash, but you haven't granted it yet.","is_error":true,"tool_use_id":"toolu_B"}]},"tool_result_meta":[{"id":"toolu_B","non_execution_kind":"user-rejected"}]}`
	resultDenialsLine   = `{"type":"result","is_error":false,"result":"","session_id":"s1","permission_denials":[{"tool_name":"Bash","tool_use_id":"toolu_A","tool_input":{"command":"rm -rf x1"}},{"tool_name":"Bash","tool_use_id":"toolu_B","tool_input":{"command":"curl x"}}]}`
)

func TestDispatch_permissionRuleMarksDenialBlocked(t *testing.T) {
	t.Parallel()
	r := &Runner{}
	var st streamState
	var denials []agent.PermissionDenial
	cb := func(e agent.Event) {
		if e.Type == agent.EventPermDenied {
			denials = e.Denials
		}
	}
	for _, line := range []string{userRuleBlockedLine, userRejectedLine, resultDenialsLine} {
		e, err := ParseEvent([]byte(line))
		if err != nil {
			t.Fatalf("ParseEvent: %v", err)
		}
		r.dispatch(e, cb, &st)
	}
	if len(denials) != 2 {
		t.Fatalf("denials=%d want 2", len(denials))
	}
	if !denials[0].BlockedByRule {
		t.Error("toolu_A 是 permission-rule，應標 BlockedByRule")
	}
	if denials[1].BlockedByRule {
		t.Error("toolu_B 是 user-rejected（需授權），不可標 BlockedByRule")
	}
}

// 沒有 tool_result_meta（舊版或欄位改名）時維持原行為：全部視為待授權。
func TestDispatch_noToolResultMetaKeepsDenialsPending(t *testing.T) {
	t.Parallel()
	r := &Runner{}
	var st streamState
	var denials []agent.PermissionDenial
	cb := func(e agent.Event) {
		if e.Type == agent.EventPermDenied {
			denials = e.Denials
		}
	}
	e, err := ParseEvent([]byte(resultDenialsLine))
	if err != nil {
		t.Fatal(err)
	}
	r.dispatch(e, cb, &st)
	if len(denials) != 2 || denials[0].BlockedByRule || denials[1].BlockedByRule {
		t.Fatalf("缺 meta 時不應標記 BlockedByRule: %+v", denials)
	}
}

func TestClaudeEnv(t *testing.T) {
	t.Parallel()
	has := func(env []string, kv string) bool { return slices.Contains(env, kv) }
	e := claudeEnv([]string{"A=1"}, false)
	if !has(e, "CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1") || has(e, "MCP_TOOL_TIMEOUT=1800000") {
		t.Fatalf("未啟用權限詢問不該動 MCP_TOOL_TIMEOUT: %v", e)
	}
	e = claudeEnv([]string{"A=1"}, true)
	if !has(e, "MCP_TOOL_TIMEOUT=1800000") {
		t.Fatalf("啟用權限詢問應放寬 MCP 工具逾時: %v", e)
	}
	e = claudeEnv([]string{"MCP_TOOL_TIMEOUT=5000"}, true)
	if has(e, "MCP_TOOL_TIMEOUT=1800000") {
		t.Fatalf("使用者自設的 MCP_TOOL_TIMEOUT 要尊重: %v", e)
	}
}

func TestBuildClaudeArgs_permPromptConfig(t *testing.T) {
	t.Parallel()
	got := buildClaudeArgs(agent.RunOptions{ExtraArgs: map[string]string{
		agent.ArgPermissionMode:   "bypassPermissions",
		agent.ArgPermPromptConfig: `C:\tmp\mcp.json`,
	}})
	want := []string{
		"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--permission-mode", "bypassPermissions",
		"--mcp-config", `C:\tmp\mcp.json`,
		"--permission-prompt-tool", "mcp__miniapp_perm__claude_permission",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}
