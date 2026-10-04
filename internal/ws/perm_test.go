package ws

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fasthttp/websocket"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/agent"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
)

// fakeKiroACP：覆寫 kiroacp runner（同 fakeClaude 的手法），每輪都發一次中途授權請求。
type fakeKiroACP struct{}

func (fakeKiroACP) Name() string { return "kiroacp" }

func (fakeKiroACP) Run(ctx context.Context, opts agent.RunOptions, cb agent.EventCallback) error {
	if opts.RequestPermission != nil {
		got := opts.RequestPermission(ctx, agent.PermissionRequest{
			Title:   "Write",
			Options: []agent.PermissionOption{{OptionID: "y", Kind: "allow_once"}, {OptionID: "n", Kind: "reject_once"}},
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if got == "y" {
			cb(agent.Event{Type: agent.EventDelta, Text: "allowed"})
		} else {
			cb(agent.Event{Type: agent.EventDelta, Text: "denied"})
		}
	}
	cb(agent.Event{Type: agent.EventDone})
	return nil
}

func init() { agent.Register(agent.TypeKiroACP, func() agent.Runner { return fakeKiroACP{} }) }

// dialCollect 連線並回傳收到 permission_request 的通知 channel。
func dialCollect(t *testing.T, url string) (*websocket.Conn, <-chan struct{}) {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	gotPerm := make(chan struct{}, 4)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m struct{ Type string }
			if json.Unmarshal(data, &m) == nil && m.Type == "permission_request" {
				gotPerm <- struct{}{}
			}
		}
	}()
	return conn, gotPerm
}

func awaitingConfirm(database *db.DB, id string) bool {
	s, _ := database.GetSession(id)
	_, waiting := permPending(id)
	return waiting && s.Status == db.SessionStatusAwaitingConfirm
}

// 回歸：kiroacp 等授權時，發起任務的連線斷了，另一條連線（重連分頁／MCP loopback）
// 必須能看到授權請求並放行；以前通道綁在發起連線的 closure，allow/deny 會被忽略而永遠卡住。
func TestKiroACPPermission_ResolvableFromOtherConnection(t *testing.T) {
	database, wsURL := startTestServer(t)
	s, err := database.CreateSession("k", "", t.TempDir(), "default", agent.TypeKiroACP, nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := dialCollect(t, wsURL(s.ID))
	if err := a.WriteJSON(map[string]string{"type": "input", "data": "go"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "awaiting confirm", func() bool { return awaitingConfirm(database, s.ID) })
	a.Close()

	b, gotPerm := dialCollect(t, wsURL(s.ID))
	defer b.Close()
	waitFor(t, "permission_request restored on reconnect", func() bool {
		select {
		case <-gotPerm:
			return true
		default:
			return false
		}
	})
	if err := b.WriteJSON(map[string]any{"type": "allow_once", "tools": []string{"Write"}}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "allowed & idle", func() bool {
		x, _ := database.GetSession(s.ID)
		return lastReply(database, s.ID) == "allowed" && x.Status == db.SessionStatusIdle && !taskIsActive(s.ID)
	})
}

// 回歸：等授權時按停止要立刻結束（子進程在等我們回覆，軟信號無效），不能停在 awaiting_confirm。
func TestKiroACPPermission_InterruptWhileWaiting(t *testing.T) {
	database, wsURL := startTestServer(t)
	s, err := database.CreateSession("k2", "", t.TempDir(), "default", agent.TypeKiroACP, nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := dialCollect(t, wsURL(s.ID))
	defer c.Close()
	if err := c.WriteJSON(map[string]string{"type": "input", "data": "go"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "awaiting confirm", func() bool { return awaitingConfirm(database, s.ID) })
	if err := c.WriteJSON(map[string]string{"type": "interrupt"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "idle after single interrupt", func() bool {
		x, _ := database.GetSession(s.ID)
		_, waiting := permPending(s.ID)
		return x.Status == db.SessionStatusIdle && !taskIsActive(s.ID) && !waiting
	})
}

// 回歸：沒有任務、卻殘留 awaiting_confirm（例如等授權時 server 重啟）時，deny/interrupt 要能清掉；
// Claude 的 pending_denials 不可被誤清。
func TestKiroACPPermission_StaleAwaitingConfirmCleared(t *testing.T) {
	database, wsURL := startTestServer(t)
	s, err := database.CreateSession("k3", "", t.TempDir(), "default", agent.TypeKiroACP, nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateSessionStatus(s.ID, db.SessionStatusAwaitingConfirm); err != nil {
		t.Fatal(err)
	}
	c, _ := dialCollect(t, wsURL(s.ID))
	defer c.Close()
	if err := c.WriteJSON(map[string]string{"type": "deny_once"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "stale cleared", func() bool {
		x, _ := database.GetSession(s.ID)
		return x.Status == db.SessionStatusIdle
	})

	// Claude 等授權（有 pending_denials）不是殘留，不可被清掉。
	cs, err := database.CreateSession("c", "", t.TempDir(), "default", agent.TypeClaude, nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	_ = database.UpdatePendingDenials(cs.ID, `[{"tool_name":"Write"}]`)
	_ = database.UpdateSessionStatus(cs.ID, db.SessionStatusAwaitingConfirm)
	if clearStaleAwaitingConfirm(database, cs.ID) {
		t.Fatal("有 pending_denials 的 Claude session 不應被當成殘留清掉")
	}
}
