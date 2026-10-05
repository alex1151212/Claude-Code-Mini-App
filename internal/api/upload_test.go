package api

import (
	"bytes"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/media"
)

func TestUpload(t *testing.T) {
	database := testDB(t)
	wd := t.TempDir()
	ws := t.TempDir()
	old := media.WorkspaceDir
	media.WorkspaceDir = ws
	t.Cleanup(func() { media.WorkspaceDir = old })
	s, err := database.CreateSession("u", "", wd, "default", "claude", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{BodyLimit: math.MaxInt32}) // 與 server.go 一致
	app.Post("/sessions/:id/uploads", NewUploadHandler(database).Upload)

	post := func(id, field, name string, body []byte) (int, map[string]string) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, _ := w.CreateFormFile(field, name)
		_, _ = fw.Write(body)
		_ = w.Close()
		req := httptest.NewRequest("POST", "/sessions/"+id+"/uploads", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	code, out := post(s.ID, "file", "shot.png", []byte("png-bytes"))
	if code != 200 {
		t.Fatalf("code=%d out=%v", code, out)
	}
	if filepath.Dir(out["path"]) != filepath.Join(ws, "uploads", s.ID) {
		t.Fatalf("path=%s", out["path"])
	}
	if b, _ := os.ReadFile(out["path"]); string(b) != "png-bytes" {
		t.Fatalf("content=%q", b)
	}

	// 不限類型與大小：.exe 與超過舊上限 8 MB 的檔案都可上傳。
	if code, _ := post(s.ID, "file", "run.exe", []byte("MZ")); code != 200 {
		t.Errorf("exe code=%d", code)
	}
	if code, _ := post(s.ID, "file", "big.bin", make([]byte, 9*1024*1024)); code != 200 {
		t.Errorf("big code=%d", code)
	}
	if code, _ := post(s.ID, "other", "a.png", []byte("x")); code != 400 {
		t.Errorf("missing field code=%d", code)
	}
	if code, _ := post("missing", "file", "a.png", []byte("x")); code != 404 {
		t.Errorf("missing session code=%d", code)
	}
}
