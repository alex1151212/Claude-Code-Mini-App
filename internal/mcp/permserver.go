package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// PermSessionHeader：claude 的 --mcp-config 以此 header 帶上自己所屬的 miniapp session id。
const PermSessionHeader = "X-Miniapp-Session"

// PermAsker 由 ws 實作：向使用者詢問一次工具授權，阻塞到使用者決定或 ctx 取消。
// allow=false 時 msg 是回給 claude 的拒絕原因。
type PermAsker func(ctx context.Context, sessionID, toolName string, input json.RawMessage, toolUseID string) (allow bool, msg string)

type permArgs struct {
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
}

// NewPermHTTPHandler 建立掛在 /mcp/perm 的 Streamable HTTP handler，只有一個工具 claude_permission，
// 給 `claude -p --permission-prompt-tool` 用。刻意不重用 /mcp 的完整 server：
// 那會把 shell_exec、delete_session 等工具一併曝給被管理的 claude session。
func NewPermHTTPHandler(ask PermAsker) http.Handler {
	s := gomcp.NewServer(&gomcp.Implementation{Name: "claude-miniapp-perm", Version: "1.0.0"}, nil)
	s.AddTool(&gomcp.Tool{
		Name:        "claude_permission",
		Description: "Claude Code 的 --permission-prompt-tool：詢問 miniapp 使用者是否允許一次工具呼叫",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool_name":   map[string]any{"type": "string"},
				"input":       map[string]any{"type": "object"},
				"tool_use_id": map[string]any{"type": "string"},
			},
			"required": []string{"tool_name", "input"},
		},
	}, func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		return permCall(ctx, req, ask)
	})
	return gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return s }, nil)
}

func permCall(ctx context.Context, req *gomcp.CallToolRequest, ask PermAsker) (*gomcp.CallToolResult, error) {
	var a permArgs
	if err := json.Unmarshal(req.Params.Arguments, &a); err != nil || a.ToolName == "" {
		return permDecision(map[string]any{"behavior": "deny", "message": "miniapp: 無法解析授權請求"}), nil
	}
	sid := ""
	if x := req.GetExtra(); x != nil && x.Header != nil {
		sid = strings.TrimSpace(x.Header.Get(PermSessionHeader))
	}
	if sid == "" {
		return permDecision(map[string]any{"behavior": "deny", "message": "miniapp: 缺少 session 識別"}), nil
	}
	allow, msg := ask(ctx, sid, a.ToolName, a.Input, a.ToolUseID)
	if !allow {
		if msg == "" {
			msg = "使用者拒絕了此操作"
		}
		return permDecision(map[string]any{"behavior": "deny", "message": msg}), nil
	}
	in := a.Input
	if len(in) == 0 {
		in = json.RawMessage(`{}`)
	}
	// updatedInput 原樣回傳，不改寫 claude 要執行的內容。
	return permDecision(map[string]any{"behavior": "allow", "updatedInput": in}), nil
}

// permDecision：permission-prompt-tool 的回傳格式是一段 JSON 文字。
func permDecision(v map[string]any) *gomcp.CallToolResult {
	b, _ := json.Marshal(v)
	return &gomcp.CallToolResult{Content: []gomcp.Content{&gomcp.TextContent{Text: string(b)}}}
}
