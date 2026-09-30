// Package mcp 讓其他 agent 透過 MCP（Streamable HTTP）操作本專案的 session。
//
// 送出/等結果分離成非阻塞設計：tool call 立刻回，實際執行狀態靠 loopback 到
// 既有 /sessions/:id/ws 的 WebSocket client 讀取、快取，get_status 再輪詢讀出。
// 不重寫 internal/ws 的執行邏輯，只是多一個呼叫方。
package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/fasthttp/websocket"
)

// 對外簡化過的狀態機，對應 internal/ws 內部各種 STATE 字串。
const (
	StateIdle               = "idle"
	StateRunning            = "running"
	StateAwaitingPermission = "awaiting_permission"
	StateShellPending       = "shell_pending"
)

type pendingShell struct {
	Command string `json:"command"`
	WorkDir string `json:"work_dir"`
}

// sessionState 是單一 session 的即時狀態快取，由 wsClient 的讀取 goroutine 更新。
type sessionState struct {
	mu                sync.Mutex
	conn              *websocket.Conn
	state             string
	latestText        string
	pendingPermission json.RawMessage
	pendingShellCmd   *pendingShell
	lastErr           string
	// writeMu 序列化 conn 寫入：websocket 只允許單一並行 writer，多個 MCP 呼叫同時操作同一 session 會寫壞 frame。
	writeMu sync.Mutex
}

func (s *sessionState) writeJSON(v any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteJSON(v)
}

func (s *sessionState) snapshot() (state, text string, perm json.RawMessage, shell *pendingShell, lastErr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.latestText, s.pendingPermission, s.pendingShellCmd, s.lastErr
}

// serverEvent 只解析我們關心的欄位，其餘忽略（鏡射 internal/ws.serverMsg）。
type serverEvent struct {
	Type    string          `json:"type"`
	Value   string          `json:"value"`
	Content string          `json:"content"`
	Tools   json.RawMessage `json:"tools"`
	Command string          `json:"command"`
	WorkDir string          `json:"work_dir"`
}

func normalizeState(v string) string {
	switch v {
	case "AWAITING_CONFIRM":
		return StateAwaitingPermission
	case "AWAITING_SHELL_CONFIRM", "SHELL_AWAITING_APPROVAL":
		return StateShellPending
	case "IDLE", "SHELL_IDLE":
		return StateIdle
	default:
		return StateRunning // THINKING / STREAMING / SHELL_RUNNING / SHELL_EXEC ...
	}
}

// Registry 管理每個 session 一條 loopback WS 連線與其狀態快取。
type Registry struct {
	mu       sync.Mutex
	sessions map[string]*sessionState
	wsURL    func(sessionID string) string // ws://127.0.0.1:<port>/sessions/:id/ws
	header   http.Header                   // 帶 mcp_token 的 Authorization header
}

func NewRegistry(wsURL func(sessionID string) string, authHeader http.Header) *Registry {
	return &Registry{
		sessions: make(map[string]*sessionState),
		wsURL:    wsURL,
		header:   authHeader,
	}
}

// ensure 回傳該 session 的狀態快取；若尚未連線就 dial 並起讀取 goroutine。
func (r *Registry) ensure(sessionID string) (*sessionState, error) {
	r.mu.Lock()
	st, ok := r.sessions[sessionID]
	if ok && st.conn != nil {
		r.mu.Unlock()
		return st, nil
	}
	r.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(r.wsURL(sessionID), r.header)
	if err != nil {
		return nil, fmt.Errorf("連線 session %s 失敗: %w", sessionID, err)
	}

	st = &sessionState{conn: conn, state: StateIdle}
	r.mu.Lock()
	// 並行呼叫時可能兩邊都 dial 了：以先登記者為準，自己這條關掉，避免兩條連線、狀態快取分裂。
	if cur, ok := r.sessions[sessionID]; ok && cur.conn != nil {
		r.mu.Unlock()
		conn.Close()
		return cur, nil
	}
	r.sessions[sessionID] = st
	r.mu.Unlock()

	go r.readLoop(sessionID, st)
	return st, nil
}

func (r *Registry) readLoop(sessionID string, st *sessionState) {
	defer func() {
		r.mu.Lock()
		if r.sessions[sessionID] == st { // 只刪自己，別刪到之後重連的新 entry
			delete(r.sessions, sessionID)
		}
		r.mu.Unlock()
		st.conn.Close()
	}()
	for {
		_, data, err := st.conn.ReadMessage()
		if err != nil {
			return
		}
		var ev serverEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		st.mu.Lock()
		switch ev.Type {
		case "status":
			st.state = normalizeState(ev.Value)
		case "delta", "thinking":
			st.latestText += ev.Content
		case "message_result_text":
			st.latestText = ev.Content
			st.state = StateIdle
		case "permission_request":
			st.pendingPermission = ev.Tools
			st.state = StateAwaitingPermission
		case "shell_command_request", "shell_approval_request":
			st.pendingShellCmd = &pendingShell{Command: ev.Command, WorkDir: ev.WorkDir}
			st.state = StateShellPending
		case "shell_delta":
			st.latestText += ev.Content
		case "shell_done":
			st.state = StateIdle
		case "shell_error", "error":
			st.lastErr = ev.Content
		}
		st.mu.Unlock()
	}
}

func (r *Registry) send(sessionID string, payload any) error {
	st, err := r.ensure(sessionID)
	if err != nil {
		return err
	}
	st.mu.Lock()
	// 新一輪操作開始：清掉上一輪殘留的文字/錯誤，讓 get_status 只看到這一輪的結果。
	st.latestText = ""
	st.lastErr = ""
	st.pendingPermission = nil
	st.state = StateRunning
	st.mu.Unlock()
	return st.writeJSON(payload)
}

// Status 回傳目前狀態快取，供 get_status tool 使用。
func (r *Registry) Status(sessionID string) (state, text string, perm json.RawMessage, shell *pendingShell, lastErr string, connected bool) {
	r.mu.Lock()
	st, ok := r.sessions[sessionID]
	r.mu.Unlock()
	if !ok {
		return StateIdle, "", nil, nil, "", false
	}
	state, text, perm, shell, lastErr = st.snapshot()
	return state, text, perm, shell, lastErr, true
}

func (r *Registry) SendMessage(sessionID, text string) error {
	return r.send(sessionID, map[string]string{"type": "input", "data": text})
}

func (r *Registry) ShellExec(sessionID, command string) error {
	return r.send(sessionID, map[string]string{"type": "shell_exec", "data": command})
}

func (r *Registry) RespondPermission(sessionID, decision string, tools []string) error {
	switch decision {
	case "allow_once":
		return r.send(sessionID, map[string]any{"type": "allow_once", "tools": tools})
	case "deny_once":
		return r.send(sessionID, map[string]string{"type": "deny_once"})
	default:
		return fmt.Errorf("未知的 decision: %s（僅支援 allow_once / deny_once）", decision)
	}
}

func (r *Registry) SetPermissionMode(sessionID, mode string) error {
	return r.send(sessionID, map[string]string{"type": "set_mode", "mode": mode})
}

func (r *Registry) SetModel(sessionID, model string) error {
	return r.send(sessionID, map[string]string{"type": "set_model", "model": model})
}

func (r *Registry) SetEffort(sessionID, effort string) error {
	return r.send(sessionID, map[string]string{"type": "set_effort", "effort": effort})
}

func (r *Registry) Interrupt(sessionID string) error {
	st, err := r.ensure(sessionID)
	if err != nil {
		return err
	}
	return st.writeJSON(map[string]string{"type": "interrupt"})
}
