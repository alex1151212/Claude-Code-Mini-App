package codex

import (
	"encoding/json"
	"errors"
)

// StreamEvent 為 codex exec --json 的 JSONL 頂層事件。
type StreamEvent struct {
	Type     string          `json:"type"`
	ThreadID string          `json:"thread_id,omitempty"`
	Item     *Item           `json:"item,omitempty"`
	Message  string          `json:"message,omitempty"`
	Usage    *TurnUsage      `json:"usage,omitempty"`
	Error    json.RawMessage `json:"error,omitempty"`
}

// Item 是 item.started / item.completed 的承載結構。
type Item struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Text    string `json:"text,omitempty"`
	Message string `json:"message,omitempty"` // item.type=error（非致命警告）
	Status  string `json:"status,omitempty"`
}

// TurnUsage 來自 turn.completed.usage。
type TurnUsage struct {
	InputTokens          int64 `json:"input_tokens"`
	CachedInputTokens    int64 `json:"cached_input_tokens"`
	OutputTokens         int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
}

// ParseEvent 解析單行 JSONL。
func ParseEvent(line []byte) (*StreamEvent, error) {
	var e StreamEvent
	if err := json.Unmarshal(line, &e); err != nil {
		return nil, err
	}
	if e.Type == "" {
		return nil, errors.New("missing event type")
	}
	return &e, nil
}

// ActivityLabel 將 item type 映射為前端活動提示文字。
func ActivityLabel(itemType string) string {
	switch itemType {
	case "command_execution":
		return "執行指令中…"
	case "web_search":
		return "搜尋中…"
	case "file_change":
		return "修改檔案中…"
	case "mcp_tool_call":
		return "呼叫工具中…"
	default:
		return ""
	}
}

// AgentMessageText 從 item.completed (agent_message) 取出文字。
func AgentMessageText(item *Item) string {
	if item == nil || item.Type != "agent_message" {
		return ""
	}
	return item.Text
}

// ErrorMessage 從 turn.failed / error 事件取出可讀訊息。
func ErrorMessage(e *StreamEvent) string {
	if e == nil {
		return "codex error"
	}
	if e.Message != "" {
		return unwrapAPIError(e.Message)
	}
	if len(e.Error) > 0 {
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(e.Error, &obj) == nil && obj.Message != "" {
			return unwrapAPIError(obj.Message)
		}
		return string(e.Error)
	}
	return "codex turn failed"
}

// unwrapAPIError：0.159.x 的 message 常是整包 API 錯誤 JSON 字串
// （{"type":"error","status":400,"error":{"message":"..."}}），取內層 message。
func unwrapAPIError(s string) string {
	var obj struct {
		Message string `json:"message"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(s), &obj) != nil {
		return s
	}
	if obj.Error != nil && obj.Error.Message != "" {
		return obj.Error.Message
	}
	if obj.Message != "" {
		return obj.Message
	}
	return s
}
