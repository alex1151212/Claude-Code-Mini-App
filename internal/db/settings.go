package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"unicode/utf8"
)

const (
	SettingKeyAppearance = "appearance"

	maxAppearanceJSON = 64 * 1024
	maxColorLen       = 128
	maxCustomCSSLen   = 32 * 1024
	maxBgImageURLLen  = 2048
)

// AppearanceDefaults 與前端 APPEARANCE_DEFAULTS 一致。
var AppearanceDefaults = map[string]any{
	"headingColor":   "oklch(0.78 0.15 65)",
	"boldColor":      "oklch(0.72 0.07 145)",
	"codeColor":      "oklch(0.63 0.12 275)",
	"customCss":      "",
	"bgImageEnabled": false,
	"bgImageUrl":     "",
}

var appearanceColorKeys = []string{"headingColor", "boldColor", "codeColor"}

// AppearanceResult 給 REST 用：外觀欄位 + 是否已寫入 DB。
type AppearanceResult struct {
	Stored bool `json:"stored"`
	Data   map[string]any
}

// MarshalJSON 把 stored 與 appearance 欄位攤平（前端讀同一層）。
func (r AppearanceResult) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(r.Data)+1)
	for k, v := range r.Data {
		out[k] = v
	}
	out["stored"] = r.Stored
	return json.Marshal(out)
}

// DefaultAppearanceMap 回傳預設外觀（可修改的 copy）。
func DefaultAppearanceMap() map[string]any {
	m := make(map[string]any, len(AppearanceDefaults))
	for k, v := range AppearanceDefaults {
		m[k] = v
	}
	return m
}

// NormalizeAppearance 解析並驗證外觀 JSON：須為 object、已知欄位為字串且長度合格；
// 缺的已知欄位補預設；多餘 key 保留。不接受 "stored" 寫入結果（會剔除）。
func NormalizeAppearance(raw []byte) (map[string]any, error) {
	if len(raw) > maxAppearanceJSON {
		return nil, fmt.Errorf("外觀 JSON 不可超過 %d bytes", maxAppearanceJSON)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("外觀須為 JSON object")
	}
	if m == nil {
		return nil, fmt.Errorf("外觀須為 JSON object")
	}
	delete(m, "stored")

	for _, key := range appearanceColorKeys {
		if err := checkStringField(m, key, maxColorLen); err != nil {
			return nil, err
		}
	}
	if err := checkStringField(m, "customCss", maxCustomCSSLen); err != nil {
		return nil, err
	}
	if v, ok := m["bgImageEnabled"]; ok {
		if _, isBool := v.(bool); !isBool {
			return nil, fmt.Errorf("bgImageEnabled 須為布林值")
		}
	}
	if err := checkStringField(m, "bgImageUrl", maxBgImageURLLen); err != nil {
		return nil, err
	}
	if s, _ := m["bgImageUrl"].(string); s != "" && !isHTTPURL(s) {
		return nil, fmt.Errorf("背景圖片網址須為 http(s) 開頭")
	}

	for k, def := range AppearanceDefaults {
		if _, ok := m[k]; !ok {
			m[k] = def
		}
	}
	return m, nil
}

func checkStringField(m map[string]any, key string, maxLen int) error {
	v, ok := m[key]
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("%s 須為字串", key)
	}
	if utf8.RuneCountInString(s) > maxLen {
		return fmt.Errorf("%s 長度不可超過 %d", key, maxLen)
	}
	return nil
}

// isHTTPURL 只接受 http(s) 絕對網址：背景圖網址會塞進 CSS url()，擋掉 javascript:、data: 等。
func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// GetAppearance 讀取 appearance；無列時回預設且 Stored=false。
func (db *DB) GetAppearance() (AppearanceResult, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, SettingKeyAppearance).Scan(&value)
	if err == sql.ErrNoRows {
		return AppearanceResult{Stored: false, Data: DefaultAppearanceMap()}, nil
	}
	if err != nil {
		return AppearanceResult{}, err
	}
	data, err := NormalizeAppearance([]byte(value))
	if err != nil {
		// DB 損壞時仍回預設，但視為已存過（避免覆寫種子誤判）
		return AppearanceResult{Stored: true, Data: DefaultAppearanceMap()}, nil
	}
	return AppearanceResult{Stored: true, Data: data}, nil
}

// PutAppearance 整包覆寫 appearance，回傳存進去的結果（Stored=true）。
func (db *DB) PutAppearance(raw []byte) (AppearanceResult, error) {
	data, err := NormalizeAppearance(raw)
	if err != nil {
		return AppearanceResult{}, err
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return AppearanceResult{}, err
	}
	if len(encoded) > maxAppearanceJSON {
		return AppearanceResult{}, fmt.Errorf("外觀 JSON 不可超過 %d bytes", maxAppearanceJSON)
	}
	_, err = db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		SettingKeyAppearance, string(encoded),
	)
	if err != nil {
		return AppearanceResult{}, err
	}
	return AppearanceResult{Stored: true, Data: data}, nil
}
