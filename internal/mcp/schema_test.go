package mcp

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// 回歸：pending_permission 以前是 json.RawMessage，schema 被推導成整數陣列，
// 實際內容（物件陣列）讓 SDK 的輸出驗證失敗，get_status 在等授權時直接報錯。
func TestStatusOutputSchemaAcceptsPermissionObjects(t *testing.T) {
	perm := rawToAny(json.RawMessage(`[{"tool_name":"WebSearch","tool_input":{"query":"x"}}]`))
	check := func(name string, schema *jsonschema.Schema, err error, out any) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		rs, err := schema.Resolve(nil)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(out)
		var v map[string]any
		_ = json.Unmarshal(b, &v)
		if err := rs.Validate(v); err != nil {
			t.Fatalf("%s 輸出驗證失敗: %v", name, err)
		}
	}
	s1, err := jsonschema.For[getStatusOut](nil)
	check("get_status", s1, err, getStatusOut{State: StateAwaitingPermission, PendingPermission: perm, Connected: true})
	s2, err := jsonschema.For[askSessionOut](nil)
	check("ask_session", s2, err, askSessionOut{State: StateAwaitingPermission, PendingPermission: perm})

	if rawToAny(nil) != nil {
		t.Fatal("空的 pending_permission 應為 nil（omitempty 才會省略）")
	}
}
