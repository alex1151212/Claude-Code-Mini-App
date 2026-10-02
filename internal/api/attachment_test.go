package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/media"
)

func TestAttachmentUploadMetadataAuthenticatedReadAndDetails(t *testing.T) {
	database := testDB(t)
	old := media.WorkspaceDir
	media.WorkspaceDir = t.TempDir()
	t.Cleanup(func() { media.WorkspaceDir = old })
	s, _ := database.CreateSession("files", "", t.TempDir(), "default", "claude", nil, "agent")
	other, _ := database.CreateSession("other", "", t.TempDir(), "default", "claude", nil, "agent")
	app := fiber.New()
	h := NewUploadHandler(database)
	protected := func(c *fiber.Ctx) error {
		if c.Get("Authorization") != "Bearer test" {
			return c.SendStatus(401)
		}
		return c.Next()
	}
	app.Post("/sessions/:id/uploads", protected, h.Upload)
	app.Get("/sessions/:id/uploads/:attachmentId/details", protected, h.Details)
	app.Get("/sessions/:id/uploads/:attachmentId", protected, h.Content)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	f, _ := w.CreateFormFile("file", `C:\fakepath\報告.txt`)
	f.Write([]byte("hello attachment"))
	w.Close()
	req := httptest.NewRequest("POST", "/sessions/"+s.ID+"/uploads", &body)
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Path       string        `json:"path"`
		Attachment db.Attachment `json:"attachment"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || out.Attachment.Name != "報告.txt" || out.Attachment.Size != 16 || out.Attachment.MimeType != "text/plain; charset=utf-8" || !filepath.IsAbs(out.Path) {
		t.Fatalf("response=%+v status=%d", out, res.StatusCode)
	}
	get := func(url string, authenticated bool) (int, []byte, string) {
		t.Helper()
		req := httptest.NewRequest("GET", url, nil)
		if authenticated {
			req.Header.Set("Authorization", "Bearer test")
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if authenticated && res.StatusCode == 200 && res.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("attachment must be private")
		}
		return res.StatusCode, data, res.Header.Get("Content-Disposition")
	}
	if status, _, _ := get(out.Attachment.URL, false); status != 401 {
		t.Fatalf("unauthenticated=%d", status)
	}
	status, data, disposition := get(out.Attachment.URL, true)
	kind, params, err := mime.ParseMediaType(disposition)
	if status != 200 || string(data) != "hello attachment" || err != nil || kind != "attachment" || params["filename"] != "報告.txt" {
		t.Fatalf("read=%d %q %s", status, data, disposition)
	}
	if status, _, _ := get(strings.Replace(out.Attachment.URL, s.ID, other.ID, 1), true); status != 404 {
		t.Fatalf("cross-session=%d", status)
	}
	status, data, _ = get(out.Attachment.URL+"/details", true)
	if status != 200 || !strings.Contains(string(data), "path") {
		t.Fatalf("details=%d %s", status, data)
	}
	if err := os.Remove(out.Path); err != nil {
		t.Fatal(err)
	}
	if status, _, _ := get(out.Attachment.URL, true); status != 404 {
		t.Fatalf("missing-file=%d", status)
	}
}

func TestAttachmentUploadMetadataFailureCleansFile(t *testing.T) {
	database := testDB(t)
	old := media.WorkspaceDir
	media.WorkspaceDir = t.TempDir()
	t.Cleanup(func() { media.WorkspaceDir = old })
	s, _ := database.CreateSession("files", "", t.TempDir(), "default", "claude", nil, "agent")
	if _, err := database.Exec(`CREATE TRIGGER fail_upload BEFORE INSERT ON attachments BEGIN SELECT RAISE(ABORT, 'simulated disk failure'); END`); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/sessions/:id/uploads", NewUploadHandler(database).Upload)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	f, _ := w.CreateFormFile("file", "file.txt")
	f.Write([]byte("data"))
	w.Close()
	req := httptest.NewRequest("POST", "/sessions/"+s.ID+"/uploads", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 500 {
		t.Fatalf("status=%d", res.StatusCode)
	}
	files, _ := os.ReadDir(filepath.Join(media.WorkspaceDir, "uploads", s.ID))
	if len(files) != 0 {
		t.Fatal("file left after metadata persistence failed")
	}
}
