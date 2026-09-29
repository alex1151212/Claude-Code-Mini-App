package db

import "testing"

func insertMsg(t *testing.T, database *DB, sessionID, role, content, createdAt string) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO messages (session_id, role, content, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		sessionID, role, content, MessageStatusDone, createdAt,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListMessagesQuery_SinceAndNewestLimit(t *testing.T) {
	database, err := Open(t.TempDir() + "/msg.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	s, err := database.CreateSession("p", "", "/tmp/p", "default", "claude", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	insertMsg(t, database, s.ID, "user", "old", "2026-09-08 10:00:00")
	insertMsg(t, database, s.ID, "user", "mid", "2026-09-09 01:00:00")
	insertMsg(t, database, s.ID, "user", "new", "2026-09-09 08:00:00")

	got, err := database.ListMessagesQuery(MessageQuery{SessionID: s.ID, Since: "2026-09-09 00:00:00", IncludeResult: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Content != "mid" || got[1].Content != "new" {
		t.Fatalf("since 應只回今天兩則，got=%v", contents(got))
	}

	got, err = database.ListMessagesQuery(MessageQuery{SessionID: s.ID, Limit: 2, IncludeResult: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Content != "mid" || got[1].Content != "new" {
		t.Fatalf("只給 limit 應取最新兩則再正序，got=%v", contents(got))
	}
}

func TestListActivity_ExcludeWorkDirSlashAndRoles(t *testing.T) {
	database, err := Open(t.TempDir() + "/act.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	eve, err := database.CreateSession("EVE", "", `C:\Users\user\Documents\EVE`, "default", "claude", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	proj, err := database.CreateSession("miniapp", "", `C:/Users/user/Documents/Repo/claude-miniapp`, "default", "cursor", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	insertMsg(t, database, eve.ID, "user", "秘書自己", "2026-09-09 08:00:00")
	insertMsg(t, database, proj.ID, "user", "修 MCP", "2026-09-09 08:10:00")
	insertMsg(t, database, proj.ID, "claude", "好的", "2026-09-09 08:10:00")

	got, err := database.ListActivity(ActivityQuery{
		Since:          "2026-09-09 00:00:00",
		Roles:          []string{"user"},
		ExcludeWorkDir: []string{"c:/users/user/documents/eve"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Content != "修 MCP" || got[0].SessionName != "miniapp" {
		t.Fatalf("應排除 EVE 且只留 user，got=%+v", got)
	}
}

func contents(msgs []*Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Content
	}
	return out
}
