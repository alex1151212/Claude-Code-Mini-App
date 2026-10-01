package media

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveUpload(t *testing.T) {
	ws := t.TempDir()
	old := WorkspaceDir
	WorkspaceDir = ws
	t.Cleanup(func() { WorkspaceDir = old })

	// 路徑穿越的原檔名只取副檔名，檔案一定落在該 session 的 uploads 目錄內。
	p, err := SaveUpload("sess1", `..\..\evil/../x.PNG`, strings.NewReader("img"))
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(ws, "uploads", "sess1")
	if filepath.Dir(p) != wantDir || filepath.Ext(p) != ".png" {
		t.Fatalf("path=%s", p)
	}
	if b, _ := os.ReadFile(p); string(b) != "img" {
		t.Fatalf("content=%q", b)
	}

	for name, r := range map[string]*bytes.Reader{
		"x.exe":   bytes.NewReader([]byte("MZ")),
		"noext":   bytes.NewReader([]byte("a")),
		"big.png": bytes.NewReader(make([]byte, MaxUploadBytes+1)),
		"e.png":   bytes.NewReader(nil),
	} {
		if _, err := SaveUpload("sess1", name, r); err == nil {
			t.Errorf("%s 應被拒絕", name)
		}
	}
	entries, _ := os.ReadDir(wantDir)
	if len(entries) != 1 {
		t.Fatalf("被拒絕的檔案不應殘留，got %d 個", len(entries))
	}
}
