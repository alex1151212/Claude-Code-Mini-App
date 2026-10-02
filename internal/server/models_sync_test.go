package server

import (
	"os"
	"slices"
	"testing"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/agent"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/claude"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
)

// PATH 清空 → claude／cursor-agent／kiro-cli 全部找不到，抓取一律失敗，不需要真的啟動任何 CLI。
func TestSyncModelOptions_ClaudeFetchFailure(t *testing.T) {
	t.Setenv("PATH", "")
	database, err := db.Open(t.TempDir() + "/models.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	// 首次安裝（DB 還沒有任何 Claude 選項）：抓不到就用內建清單當種子。
	failed := syncModelOptions(database)
	got, _ := database.ListModelOptions(agent.TypeClaude)
	if len(got) != len(claude.ModelOptions()) {
		t.Fatalf("首次安裝應以內建清單當種子：got %d 筆，want %d", len(got), len(claude.ModelOptions()))
	}
	if slices.Contains(failed, agent.TypeClaude) {
		t.Errorf("已用種子補上，claude 不該算失敗：%v", failed)
	}
	if !slices.Contains(failed, agent.TypeCursor) {
		t.Errorf("cursor 找不到執行檔，應列入失敗：%v", failed)
	}

	// 已有選項（例如先前抓到的即時清單）：暫時抓不到不能把它洗掉，並回報 claude 失敗。
	if err := database.SyncModelOptions(agent.TypeClaude, []db.ModelOption{{ModelID: "claude-live-only", Label: "Live"}}); err != nil {
		t.Fatal(err)
	}
	failed = syncModelOptions(database)
	got, _ = database.ListModelOptions(agent.TypeClaude)
	if len(got) != 1 || got[0].ModelID != "claude-live-only" {
		t.Fatalf("抓取失敗時不該動既有清單：%+v", got)
	}
	if !slices.Contains(failed, agent.TypeClaude) {
		t.Errorf("claude 抓取失敗應回報：%v", failed)
	}
}

// 實機測試：真的啟動各 CLI 抓清單（數秒，並觸發使用者的 hooks），預設跳過。
//
//	CLAUDE_LIVE_TEST=1 go test ./internal/server -run Live -v
func TestSyncModelOptions_Live(t *testing.T) {
	if os.Getenv("CLAUDE_LIVE_TEST") == "" {
		t.Skip("設 CLAUDE_LIVE_TEST=1 才會啟動真的 CLI")
	}
	database, err := db.Open(t.TempDir() + "/models.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	failed := syncModelOptions(database)
	got, _ := database.ListModelOptions(agent.TypeClaude)
	t.Logf("failed=%v，claude 選項 %d 筆", failed, len(got))
	if slices.Contains(failed, agent.TypeClaude) {
		t.Fatal("claude 即時抓取失敗")
	}
	builtin := make(map[string]bool)
	for _, e := range claude.ModelOptions() {
		builtin[e.ModelID] = true
	}
	// 即時清單與內建清單不同（例如 haiku 的完整 ID），表示資料真的來自 claude 而不是種子。
	for _, o := range got {
		if o.ModelID == "default" {
			t.Errorf("不該有 default：%+v", o)
		}
		t.Logf("  %s (%s) 內建清單有=%v", o.ModelID, o.Label, builtin[o.ModelID])
	}
}
