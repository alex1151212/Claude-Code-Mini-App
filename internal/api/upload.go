package api

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/media"
)

// UploadHandler stores private uploads and exposes authenticated metadata/content.
type UploadHandler struct {
	db *db.DB
}

func NewUploadHandler(database *db.DB) *UploadHandler {
	return &UploadHandler{db: database}
}

// Upload POST /sessions/:id/uploads，multipart 欄位 file。
func (h *UploadHandler) Upload(c *fiber.Ctx) error {
	sess, err := h.db.GetSession(c.Params("id"))
	if err != nil {
		return jsonErr(c, 404, "session 不存在")
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return jsonErr(c, 400, "缺少 file 欄位")
	}
	f, err := fh.Open()
	if err != nil {
		return jsonErr(c, 400, "讀取上傳檔失敗")
	}
	defer f.Close()
	path, err := media.SaveUpload(sess.ID, fh.Filename, f)
	if err != nil {
		if strings.HasPrefix(err.Error(), "media:") {
			return jsonErr(c, 500, "儲存檔案失敗")
		}
		return jsonErr(c, 400, err.Error())
	}
	st, err := os.Stat(path)
	if err != nil {
		_ = os.Remove(path)
		return jsonErr(c, 500, "讀取檔案失敗")
	}
	// Normalize Windows and Unix client names; never use them as storage paths.
	name := filepath.Base(strings.ReplaceAll(fh.Filename, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." {
		name = "附件" + filepath.Ext(path)
	}
	// Sniff content on the server; client MIME declarations are not trusted.
	buf := make([]byte, 512)
	saved, err := os.Open(path)
	if err != nil {
		_ = os.Remove(path)
		return jsonErr(c, 500, "讀取檔案失敗")
	}
	n, _ := saved.Read(buf)
	saved.Close()
	a := &db.Attachment{ID: uuid.NewString(), SessionID: sess.ID, Name: name, StorageName: filepath.Base(path), MimeType: http.DetectContentType(buf[:n]), Size: st.Size()}
	if err := h.db.AddAttachment(a); err != nil {
		_ = os.Remove(path)
		return jsonErr(c, 500, "儲存附件資訊失敗")
	}
	a, err = h.db.GetAttachment(sess.ID, a.ID)
	if err != nil {
		return jsonErr(c, 500, "讀取附件資訊失敗")
	}
	// path/name remain for older clients; new clients send attachment IDs only.
	return c.JSON(fiber.Map{"path": path, "name": a.Name, "attachment": a})
}

func (h *UploadHandler) attachment(c *fiber.Ctx) (*db.Attachment, string, error) {
	if _, err := h.db.GetSession(c.Params("id")); err != nil {
		return nil, "", fiber.NewError(404, "session 不存在")
	}
	a, err := h.db.GetAttachment(c.Params("id"), c.Params("attachmentId"))
	if err != nil {
		return nil, "", fiber.NewError(404, "附件不存在")
	}
	p, err := media.ExistingUploadPath(a.SessionID, a.StorageName)
	if err != nil {
		return nil, "", fiber.NewError(404, "附件無法讀取，請重新上傳")
	}
	return a, p, nil
}

// Content must be registered behind the same auth middleware as uploads.
func (h *UploadHandler) Content(c *fiber.Ctx) error {
	a, p, err := h.attachment(c)
	if err != nil {
		return err
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Type", a.MimeType)
	disposition := "attachment"
	if strings.HasPrefix(a.MimeType, "image/") && c.Query("download") != "1" {
		disposition = "inline"
	}
	c.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.Name}))
	// SendFile keeps cached file handles open, which locks uploads on Windows.
	// SendStream owns and closes this reader when the response is consumed.
	f, err := os.Open(p)
	if err != nil {
		return fiber.NewError(404, "附件無法讀取")
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return fiber.NewError(404, "附件無法讀取")
	}
	return c.SendStream(f, int(st.Size()))
}

func (h *UploadHandler) Details(c *fiber.Ctx) error {
	a, p, err := h.attachment(c)
	if err != nil {
		return err
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"attachment": a, "path": p})
}
