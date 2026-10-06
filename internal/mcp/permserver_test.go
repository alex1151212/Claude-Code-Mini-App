package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type headerRT struct{ h map[string]string }

func (r headerRT) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range r.h {
		req.Header.Set(k, v)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func callPerm(t *testing.T, ask PermAsker, headers map[string]string, args map[string]any) map[string]any {
	t.Helper()
	srv := httptest.NewServer(NewPermHTTPHandler(ask))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := gomcp.NewClient(&gomcp.Implementation{Name: "t", Version: "1"}, nil)
	sess, err := client.Connect(ctx, &gomcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		HTTPClient:           &http.Client{Transport: headerRT{headers}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()
	tools, err := sess.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "claude_permission" {
		t.Fatalf("應只暴露 claude_permission 一個工具: %v %+v", err, tools)
	}
	res, err := sess.CallTool(ctx, &gomcp.CallToolParams{Name: "claude_permission", Arguments: args})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("content=%v", res.Content)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].(*gomcp.TextContent).Text), &out); err != nil {
		t.Fatalf("回傳不是 JSON 決定: %v", err)
	}
	return out
}

func TestPermServer_allowEchoesInputAndCarriesSession(t *testing.T) {
	t.Parallel()
	var gotSid, gotTool, gotID string
	var gotInput json.RawMessage
	ask := func(_ context.Context, sid, tool string, in json.RawMessage, id string) (bool, string) {
		gotSid, gotTool, gotInput, gotID = sid, tool, in, id
		return true, ""
	}
	out := callPerm(t, ask, map[string]string{PermSessionHeader: "sess-1"},
		map[string]any{"tool_name": "Bash", "input": map[string]any{"command": "rm -rf x"}, "tool_use_id": "toolu_1"})
	if out["behavior"] != "allow" {
		t.Fatalf("out=%v", out)
	}
	if up, _ := out["updatedInput"].(map[string]any); up["command"] != "rm -rf x" {
		t.Errorf("updatedInput 應原樣回傳: %v", out["updatedInput"])
	}
	if gotSid != "sess-1" || gotTool != "Bash" || gotID != "toolu_1" || string(gotInput) != `{"command":"rm -rf x"}` {
		t.Errorf("ask 收到 sid=%q tool=%q id=%q input=%s", gotSid, gotTool, gotID, gotInput)
	}
}

func TestPermServer_denyCarriesMessage(t *testing.T) {
	t.Parallel()
	ask := func(context.Context, string, string, json.RawMessage, string) (bool, string) { return false, "nope" }
	out := callPerm(t, ask, map[string]string{PermSessionHeader: "s"},
		map[string]any{"tool_name": "Bash", "input": map[string]any{"command": "curl x"}})
	if out["behavior"] != "deny" || out["message"] != "nope" {
		t.Fatalf("out=%v", out)
	}
}

func TestPermServer_missingSessionDeniesWithoutAsking(t *testing.T) {
	t.Parallel()
	asked := false
	ask := func(context.Context, string, string, json.RawMessage, string) (bool, string) { asked = true; return true, "" }
	out := callPerm(t, ask, nil, map[string]any{"tool_name": "Bash", "input": map[string]any{}})
	if out["behavior"] != "deny" || asked {
		t.Fatalf("缺 session header 必須直接拒絕且不詢問: out=%v asked=%v", out, asked)
	}
}
