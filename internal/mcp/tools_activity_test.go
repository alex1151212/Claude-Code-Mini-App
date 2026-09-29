package mcp

import (
	"testing"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
)

func TestListActivity_RequiresSinceAndGroups(t *testing.T) {
	database, err := db.Open(t.TempDir() + "/mcp_act.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	s, err := database.CreateSession("miniapp", "", "/tmp/miniapp", "default", "cursor", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(
		`INSERT INTO messages (session_id, role, content, status, created_at) VALUES (?, 'user', '修 MCP', 'done', '2026-09-09 08:00:00')`,
		s.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	d := &deps{db: database}
	_, _, err = d.listActivity(t.Context(), nil, listActivityIn{})
	if err == nil {
		t.Fatal("since 為空應失敗")
	}

	_, out, err := d.listActivity(t.Context(), nil, listActivityIn{
		Since: "2026-09-09T00:00:00+08:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Sessions) != 1 || out.Sessions[0].Name != "miniapp" || out.Sessions[0].Count != 1 {
		t.Fatalf("got %+v", out.Sessions)
	}
	if out.Sessions[0].Messages[0].Content != "修 MCP" {
		t.Fatalf("content=%q", out.Sessions[0].Messages[0].Content)
	}

	_, msgs, err := d.getMessages(t.Context(), nil, getMessagesIn{
		SessionID: s.ID,
		Since:     "2026-09-09T00:00:00+08:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs.Messages) != 1 {
		t.Fatalf("get_messages since 應回 1 則，got %d", len(msgs.Messages))
	}
}
