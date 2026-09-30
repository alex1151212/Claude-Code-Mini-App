package mcp

import (
	"fmt"
	"net/http"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/quota"
)

// NewHTTPHandler 建立掛在 /mcp 的 Streamable HTTP handler。
// selfPort 是本服務自己監聽的 port，用來 loopback 撥打 /sessions/:id/ws。
// mcpToken 會原樣帶進 loopback WS 連線的 Authorization header，讓它通過既有 authMiddleware。
// maxHops 是 session 互問跳數上限；<=0 用 DefaultMaxHops。
func NewHTTPHandler(database *db.DB, quotaSvc *quota.Service, selfPort int, mcpToken string, maxHops int) http.Handler {
	d := &deps{
		db:      database,
		quota:   quotaSvc,
		maxHops: maxHops,
	}
	header := http.Header{}
	if mcpToken != "" {
		header.Set("Authorization", "Bearer "+mcpToken)
	}
	d.reg = NewRegistry(func(sessionID string) string {
		return fmt.Sprintf("ws://127.0.0.1:%d/sessions/%s/ws", selfPort, sessionID)
	}, header)

	server := gomcp.NewServer(&gomcp.Implementation{Name: "claude-miniapp", Version: "1.0.0"}, nil)
	registerTools(server, d)

	return gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return server }, nil)
}
