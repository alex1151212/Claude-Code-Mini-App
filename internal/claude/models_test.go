package claude

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

const reqID = "models-1"

func controlResponse(subtype, requestID, body string) []byte {
	return []byte(fmt.Sprintf(
		`{"type":"control_response","response":{"subtype":%q,"request_id":%q,%s}}`+"\n", subtype, requestID, body))
}

func TestParseInitializeModels_DedupesAndSkipsDefault(t *testing.T) {
	// 取自 POC 的真實回應（刪減）：default 與 sonnet 的 resolvedModel 相同。
	line := controlResponse("success", reqID, `"response":{"commands":[],"models":[
		{"value":"default","resolvedModel":"claude-sonnet-5-5","displayName":"Default (recommended)"},
		{"value":"opus","resolvedModel":"claude-opus-5-5","displayName":"Opus 5.5"},
		{"value":"sonnet","resolvedModel":"claude-sonnet-5-5","displayName":"Sonnet 5.5"},
		{"value":"haiku","resolvedModel":"claude-haiku-4-5-20251001","displayName":"Haiku 4.5"},
		{"value":"claude-opus-4-6","resolvedModel":"claude-opus-4-6","displayName":""}]}`)
	entries, matched, err := parseInitializeModels(line, reqID)
	if !matched || err != nil {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.ModelID+"="+e.Label)
	}
	want := "claude-opus-5-5=Opus 5.5,claude-sonnet-5-5=Sonnet 5.5,claude-haiku-4-5-20251001=Haiku 4.5,claude-opus-4-6=claude-opus-4-6"
	if strings.Join(got, ",") != want {
		t.Fatalf("got  %s\nwant %s", strings.Join(got, ","), want)
	}
}

func TestParseInitializeModels_IgnoresUnrelatedLines(t *testing.T) {
	for name, line := range map[string][]byte{
		"hook 事件":        []byte(`{"type":"system","subtype":"hook_started"}` + "\n"),
		"別的 request_id":  controlResponse("success", "other", `"response":{"models":[{"value":"a","resolvedModel":"a"}]}`),
		"含關鍵字但不是合法 JSON": []byte(`"control_response" {not json` + "\n"),
		"空行":             nil,
	} {
		if _, matched, err := parseInitializeModels(line, reqID); matched || err != nil {
			t.Errorf("%s：不該被當成回應（matched=%v err=%v）", name, matched, err)
		}
	}
}

func TestParseInitializeModels_Errors(t *testing.T) {
	for name, line := range map[string][]byte{
		"subtype=error": controlResponse("error", reqID, `"error":"boom"`),
		"models 為空":     controlResponse("success", reqID, `"response":{"models":[]}`),
		"沒有 models 欄位":  controlResponse("success", reqID, `"response":{}`),
		"只有 default":    controlResponse("success", reqID, `"response":{"models":[{"value":"default","resolvedModel":"x"}]}`),
	} {
		entries, matched, err := parseInitializeModels(line, reqID)
		if !matched || err == nil || entries != nil {
			t.Errorf("%s：應為 matched 且回 error（matched=%v err=%v entries=%v）", name, matched, err, entries)
		}
	}
}

// 實機測試：會真的啟動 claude（約 1～2 秒，並觸發使用者的 hooks），預設跳過。
//
//	CLAUDE_LIVE_TEST=1 go test ./internal/claude -run Live -v
func TestFetchModelOptions_Live(t *testing.T) {
	if os.Getenv("CLAUDE_LIVE_TEST") == "" {
		t.Skip("設 CLAUDE_LIVE_TEST=1 才會啟動真的 claude")
	}
	entries, err := FetchModelOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.ModelID] || e.ModelID == "default" || e.Label == "" {
			t.Errorf("不該出現重複／default／空 label：%+v", e)
		}
		seen[e.ModelID] = true
	}
	t.Logf("取得 %d 個模型：%v", len(entries), entries)
}
