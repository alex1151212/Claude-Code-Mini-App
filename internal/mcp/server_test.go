package mcp

import (
	"testing"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
)

// NewHTTPHandler 會 gomcp.AddTool 所有工具；schema tag 寫錯會在 server 啟動時 panic（單元測試不會經過）。
func TestNewHTTPHandlerRegistersTools(t *testing.T) {
	database, err := db.Open(t.TempDir() + "/reg.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if NewHTTPHandler(database, nil, 1, "tok", 0) == nil {
		t.Fatal("handler nil")
	}
}
