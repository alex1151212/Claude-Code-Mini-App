package db

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetAppearance_MissingRow(t *testing.T) {
	database, err := Open(t.TempDir() + "/settings.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	res, err := database.GetAppearance()
	if err != nil {
		t.Fatal(err)
	}
	if res.Stored {
		t.Fatal("缺列時 stored 應為 false")
	}
	if res.Data["headingColor"] != AppearanceDefaults["headingColor"] {
		t.Fatalf("headingColor=%v", res.Data["headingColor"])
	}
}

func TestPutAppearance_Roundtrip(t *testing.T) {
	database, err := Open(t.TempDir() + "/settings_rt.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	body := []byte(`{"headingColor":"#ffaa00","boldColor":"oklch(0.7 0.1 145)","codeColor":"#abc","customCss":".x{color:red}","extra":"keep"}`)
	put, err := database.PutAppearance(body)
	if err != nil {
		t.Fatal(err)
	}
	if !put.Stored {
		t.Fatal("Put 後 stored 應為 true")
	}
	if put.Data["headingColor"] != "#ffaa00" {
		t.Fatalf("headingColor=%v", put.Data["headingColor"])
	}
	if put.Data["extra"] != "keep" {
		t.Fatalf("應保留多餘 key，extra=%v", put.Data["extra"])
	}

	got, err := database.GetAppearance()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Stored {
		t.Fatal("有列時 stored 應為 true")
	}
	if got.Data["customCss"] != ".x{color:red}" {
		t.Fatalf("customCss=%v", got.Data["customCss"])
	}
	if got.Data["extra"] != "keep" {
		t.Fatalf("Get 應保留多餘 key，extra=%v", got.Data["extra"])
	}
}

func TestPutAppearance_RejectsOversized(t *testing.T) {
	database, err := Open(t.TempDir() + "/settings_big.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	big := map[string]any{
		"headingColor": AppearanceDefaults["headingColor"],
		"boldColor":    AppearanceDefaults["boldColor"],
		"codeColor":    AppearanceDefaults["codeColor"],
		"customCss":    strings.Repeat("a", maxCustomCSSLen+1),
	}
	raw, _ := json.Marshal(big)
	if _, err := database.PutAppearance(raw); err == nil {
		t.Fatal("customCss 超長應拒絕")
	}

	raw = []byte(`{"headingColor":` + `"` + strings.Repeat("x", maxColorLen+1) + `"}`)
	if _, err := database.PutAppearance(raw); err == nil {
		t.Fatal("headingColor 超長應拒絕")
	}
}

func TestPutAppearance_RejectsEmptyBody(t *testing.T) {
	database, err := Open(t.TempDir() + "/settings_empty.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.PutAppearance(nil); err == nil {
		t.Fatal("空 body 應拒絕，不可靜默寫入預設值")
	}
}

func TestPutAppearance_RejectsNonStringColor(t *testing.T) {
	database, err := Open(t.TempDir() + "/settings_type.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.PutAppearance([]byte(`{"headingColor":123}`)); err == nil {
		t.Fatal("非字串顏色應拒絕")
	}
}

func TestPutAppearance_BgImage(t *testing.T) {
	database, err := Open(t.TempDir() + "/settings_bg.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	// 舊資料沒有背景圖欄位：補預設（關閉、空網址）。
	got, err := database.PutAppearance([]byte(`{"customCss":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Data["bgImageEnabled"] != false || got.Data["bgImageUrl"] != "" {
		t.Fatalf("預設背景圖欄位錯誤：%v / %v", got.Data["bgImageEnabled"], got.Data["bgImageUrl"])
	}

	got, err = database.PutAppearance([]byte(`{"bgImageEnabled":true,"bgImageUrl":"https://i.imgur.com/abc.png"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Data["bgImageEnabled"] != true || got.Data["bgImageUrl"] != "https://i.imgur.com/abc.png" {
		t.Fatalf("背景圖欄位未保存：%v / %v", got.Data["bgImageEnabled"], got.Data["bgImageUrl"])
	}

	for _, body := range []string{
		`{"bgImageEnabled":"true"}`,
		`{"bgImageUrl":"javascript:alert(1)"}`,
		`{"bgImageUrl":"data:image/png;base64,AAAA"}`,
		`{"bgImageUrl":"/relative.png"}`,
		`{"bgImageUrl":"https://` + strings.Repeat("a", maxBgImageURLLen) + `"}`,
	} {
		if _, err := database.PutAppearance([]byte(body)); err == nil {
			t.Fatalf("應拒絕：%s", body)
		}
	}
}

func TestAppearanceResult_MarshalFlat(t *testing.T) {
	res := AppearanceResult{
		Stored: true,
		Data:   map[string]any{"headingColor": "#fff", "boldColor": "#000", "codeColor": "#111", "customCss": ""},
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["stored"] != true {
		t.Fatalf("stored=%v", m["stored"])
	}
	if m["headingColor"] != "#fff" {
		t.Fatalf("headingColor=%v", m["headingColor"])
	}
}
