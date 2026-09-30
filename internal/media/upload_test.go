package media

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveUpload(t *testing.T) {
	wd := t.TempDir()

	// 路徑穿越的原檔名只取副檔名，檔案一定落在 uploads 目錄內。
	p, err := SaveUpload(wd, `..\..\evil/../x.PNG`, strings.NewReader("img"))
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(wd, ".miniapp", "uploads")
	if filepath.Dir(p) != wantDir || filepath.Ext(p) != ".png" {
		t.Fatalf("path=%s", p)
	}
	if b, _ := os.ReadFile(p); string(b) != "img" {
		t.Fatalf("content=%q", b)
	}

	// .miniapp/.gitignore 忽略自身全部內容；已存在時不覆寫。
	ignore := filepath.Join(wd, ".miniapp", ".gitignore")
	if b, _ := os.ReadFile(ignore); string(b) != "*\n" {
		t.Fatalf(".gitignore=%q", b)
	}
	_ = os.WriteFile(ignore, []byte("custom\n"), 0o644)
	if _, err := SaveUpload(wd, "a.txt", strings.NewReader("t")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(ignore); string(b) != "custom\n" {
		t.Fatalf("既有 .gitignore 不應被覆寫: %q", b)
	}

	for name, r := range map[string]*bytes.Reader{
		"x.exe":   bytes.NewReader([]byte("MZ")),
		"noext":   bytes.NewReader([]byte("a")),
		"big.png": bytes.NewReader(make([]byte, MaxUploadBytes+1)),
		"e.png":   bytes.NewReader(nil),
	} {
		if _, err := SaveUpload(wd, name, r); err == nil {
			t.Errorf("%s 應被拒絕", name)
		}
	}
	entries, _ := os.ReadDir(wantDir)
	if len(entries) != 2 {
		t.Fatalf("被拒絕的檔案不應殘留，got %d 個", len(entries))
	}
}
