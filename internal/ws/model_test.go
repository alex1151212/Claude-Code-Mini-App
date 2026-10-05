package ws

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/agent"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
)

func TestSessionModelPayload_AntigravityNil(t *testing.T) {
	sess := &db.Session{AgentType: agent.TypeAntigravity}
	if sessionModelPayload(sess) != nil {
		t.Fatal("antigravity should not expose model payload")
	}
}

func TestSessionModelPayload_CursorDefault(t *testing.T) {
	sess := &db.Session{AgentType: agent.TypeCursor, CliExtraArgs: []string{}}
	p := sessionModelPayload(sess)
	if p == nil || p.DisplayText != "auto" {
		t.Fatalf("got %+v", p)
	}
}

func TestPersistModelUpdate(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	sess, err := database.CreateSession("m", "", "", "default", agent.TypeClaude, nil, "agent")
	if err != nil {
		t.Fatal(err)
	}

	p := persistModelUpdate(database, sess.ID, &agent.ModelSnapshot{
		Model:       "claude-sonnet-5",
		DisplayText: "claude-sonnet-5",
		Source:      "init_event",
	})
	if p.DisplayText != "claude-sonnet-5" || p.Source != "init_event" {
		t.Fatalf("got %+v", p)
	}

	updated, err := database.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ActiveModel != "claude-sonnet-5" || updated.ActiveModelSource != "init_event" {
		t.Fatalf("db: %+v", updated)
	}
}


// 回歸：下拉選的 sess.Model 必須壓過舊版寫壞的 ActiveModel 與 config.toml 預設。
func TestSessionModelPayload_StreamlessPrefersSelectedModel(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte("model = \"gpt-6-astra\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOME", dir)

	sess := &db.Session{AgentType: agent.TypeCodex, Model: "gpt-6.1-sol", ActiveModel: "gpt-6-astra", ActiveModelSource: "global_config"}
	if p := sessionModelPayload(sess); p == nil || p.DisplayText != "gpt-6.1-sol" {
		t.Fatalf("selected model: got %+v", p)
	}
	if info := resolveStreamless(&db.Session{AgentType: agent.TypeCodex, Model: "gpt-6.1-sol", CliExtraArgs: []string{"--model", "x"}}, agent.TypeCodex); info.Model != "gpt-6.1-sol" {
		t.Fatalf("sess.Model should beat cli_extra_args: got %+v", info)
	}
	// 未選擇時退回全域設定。
	if info := resolveStreamless(&db.Session{AgentType: agent.TypeCodex}, agent.TypeCodex); info.Model != "gpt-6-astra" {
		t.Fatalf("fallback: got %+v", info)
	}
	// claude 不受影響：stream 回填的 ActiveModel 照舊優先。
	c := &db.Session{AgentType: agent.TypeClaude, Model: "opus", ActiveModel: "claude-sonnet-5", ActiveModelSource: "init_event"}
	if p := sessionModelPayload(c); p == nil || p.DisplayText != "claude-sonnet-5" {
		t.Fatalf("claude: got %+v", p)
	}
}
