package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
)

// fakeSessionWS 模擬 /sessions/:id/ws：收到 input 後依內容推播事件。
//   - "slow"：只推 THINKING + 部分 delta，不結束（測 timeout）
//   - "perm"：推 permission_request（等授權也算本輪結束）
//   - 其他：THINKING → delta → message_result_text → IDLE
func fakeSessionWS(t *testing.T) *httptest.Server {
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			var in map[string]string
			if err := c.ReadJSON(&in); err != nil {
				return
			}
			if in["type"] != "input" {
				continue
			}
			emit := func(v map[string]any) { _ = c.WriteJSON(v) }
			emit(map[string]any{"type": "status", "value": "THINKING"})
			emit(map[string]any{"type": "quota_update"}) // 無關事件不可提早結束本輪
			switch {
			case strings.Contains(in["data"], "slow"):
				emit(map[string]any{"type": "delta", "content": "partial"})
			case strings.Contains(in["data"], "perm"):
				emit(map[string]any{"type": "status", "value": "AWAITING_CONFIRM"}) // 真實 handler 先送 status 再送內容
				emit(map[string]any{"type": "permission_request", "tools": []map[string]string{{"tool_name": "Write"}}})
			default:
				emit(map[string]any{"type": "delta", "content": "ans"})
				time.Sleep(30 * time.Millisecond)
				emit(map[string]any{"type": "message_result_text", "content": "answer: " + in["data"]})
				emit(map[string]any{"type": "status", "value": "IDLE"})
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRegistryAsk(t *testing.T) {
	srv := fakeSessionWS(t)
	reg := NewRegistry(func(string) string { return "ws" + strings.TrimPrefix(srv.URL, "http") }, http.Header{})
	ctx := context.Background()

	res, err := reg.Ask(ctx, "s1", "hello", 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.TimedOut || res.State != StateIdle || res.Text != "answer: hello" {
		t.Fatalf("完成應回全文: %+v", res)
	}

	res, err = reg.Ask(ctx, "s1", "perm", 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.TimedOut || res.State != StateAwaitingPermission || len(res.PendingPermission) == 0 {
		t.Fatalf("等授權應立即回傳: %+v", res)
	}

	start := time.Now()
	res, err = reg.Ask(ctx, "s1", "slow", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut || res.State != StateRunning || res.Text != "partial" {
		t.Fatalf("逾時應回部分文字: %+v", res)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout 未生效")
	}

	cctx, cancel := context.WithCancel(ctx)
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	if _, err := reg.Ask(cctx, "s2", "slow", 5*time.Second); err == nil {
		t.Fatal("ctx 取消應回錯誤")
	}

	// 等待中被同 session 的其他 send 取代：不可假裝成功，要帶錯誤說明。
	go func() { time.Sleep(50 * time.Millisecond); _ = reg.SetModel("s3", "x") }()
	res, err = reg.Ask(ctx, "s3", "slow", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.TimedOut || res.Error == "" {
		t.Fatalf("被取代應回錯誤說明: %+v", res)
	}
}
