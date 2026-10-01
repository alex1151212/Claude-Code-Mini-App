package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/media"
)

// UploadHandler 讓使用者上傳圖片／檔案給 agent：存進 runtime 的 workspace/uploads/<session_id>/，回傳絕對路徑，
// 由前端把路徑放進 prompt（各家 CLI 都能用路徑讀檔，不需處理各自的多模態格式）。
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
	if fh.Size > media.MaxUploadBytes {
		return jsonErr(c, 413, "檔案超過上限 8 MB")
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
	return c.JSON(fiber.Map{"path": path, "name": fh.Filename})
}
