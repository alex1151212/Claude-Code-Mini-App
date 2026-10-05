package claude

import "testing"

func TestParseEvent_ToolResultStringContent(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":"plain text"},{"type":"tool_result","content":[{"type":"image","source":{"type":"base64","data":"AAA"}}]}]}}`
	e, err := ParseEvent([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(e.Images()); got != 1 {
		t.Fatalf("images = %d, want 1", got)
	}
}
