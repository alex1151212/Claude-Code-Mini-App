package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/db"
)

// guestToken 取出訪客 token：Header X-Share-Token，或 query ?share=（WebSocket 無法自訂 Header）。
func guestToken(c *fiber.Ctx) string {
	if t := strings.TrimSpace(c.Get("X-Share-Token")); t != "" {
		return t
	}
	return strings.TrimSpace(c.Query("share"))
}

func guestDeny(c *fiber.Ctx, status int, code, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg, "code": code})
}

// guestAuth 是 authMiddleware 的訪客分支：
//  1. 以 guest token 查出分享，並於「每次請求」檢查撤銷／到期。
//  2. 路由範圍檢查：只能碰綁定 session 的 ws／messages／uploads（細節見 guestRouteAllowed）。
//  3. 通過後把身分寫入 Locals（刻意不寫 tg_id，訪客不會觸發 Telegram 通知）。
func guestAuth(database *db.DB, c *fiber.Ctx, token string) error {
	ga, err := database.ResolveGuestToken(token)
	if err != nil {
		if errors.Is(err, db.ErrShareNotFound) {
			return guestDeny(c, http.StatusUnauthorized, "share_invalid", "分享連結已失效")
		}
		slog.Warn("[auth] 訪客 token 查詢失敗", "err", err)
		return guestDeny(c, http.StatusInternalServerError, "db_error", "DB 錯誤")
	}
	sh := &ga.Share
	if !sh.Active(time.Now()) {
		return guestDeny(c, http.StatusUnauthorized, "share_ended", "分享已結束")
	}
	allowed, err := guestRouteAllowed(database, c, ga)
	if err != nil {
		slog.Warn("[auth] 訪客範圍檢查失敗", "err", err)
		return guestDeny(c, http.StatusInternalServerError, "db_error", "DB 錯誤")
	}
	if !allowed {
		slog.Warn("[auth] 訪客越權存取被拒", "share_id", sh.ID, "nickname", ga.Guest.Nickname, "method", c.Method(), "path", c.Path())
		return guestDeny(c, http.StatusForbidden, "share_forbidden", "此分享無權執行該操作")
	}

	c.Locals("share_id", sh.ID)
	c.Locals("share_session_id", sh.SessionID)
	c.Locals("share_role", sh.Role)
	c.Locals("share_mode", sh.Mode)
	c.Locals("share_nickname", ga.Guest.Nickname)
	c.Locals("share_expires_at", sh.ExpiresAt)
	c.Locals("share_guest_token", ga.Guest.Token)
	if sh.Mode == db.ShareModeSnapshot && sh.SnapshotMsgID != nil {
		// Messages handler 讀這個值強制附加 id <= snapshot_msg_id。
		c.Locals("share_snapshot_msg_id", *sh.SnapshotMsgID)
	}
	return c.Next()
}

// guestRouteAllowed 依「路由樣式」（而非原始 URL，避免路徑正規化繞過）與分享的角色／範圍判斷是否放行：
//
//	GET  /guest/me                                   所有訪客
//	GET  /sessions/:id/messages                      所有訪客（snapshot 由 handler 附加 id <= snapshot_msg_id）
//	GET  /sessions/:id/uploads/:attachmentId         所有訪客（snapshot 僅限快照內訊息引用的附件）
//	GET  /sessions/:id/ws                            live（viewer 單向、editor 雙向，由 ws handler 強制）
//	POST /sessions/:id/uploads                       editor
//	GET  /model-options/:agentType                   editor（唯讀選項清單，供模型選單使用）
//
// 其餘一律 403；含 /uploads/:attachmentId/details（會洩漏伺服器檔案路徑）。
// 凡帶 :id 的路由，:id 必須等於 token 綁定的 session。
func guestRouteAllowed(database *db.DB, c *fiber.Ctx, ga *db.GuestAccess) (bool, error) {
	sh := &ga.Share
	route := c.Route().Path
	isRead := c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead

	switch route {
	case "/guest/me":
		return isRead, nil
	case "/model-options/:agentType":
		return isRead && sh.Role == db.ShareRoleEditor, nil
	}

	// 以下路由都綁定 session
	if c.Params("id") != sh.SessionID {
		return false, nil
	}
	switch route {
	case "/sessions/:id/messages":
		return isRead, nil
	case "/sessions/:id/ws":
		return isRead && sh.Mode == db.ShareModeLive, nil
	case "/sessions/:id/uploads":
		return c.Method() == fiber.MethodPost && sh.Role == db.ShareRoleEditor && sh.Mode == db.ShareModeLive, nil
	case "/sessions/:id/uploads/:attachmentId":
		if !isRead {
			return false, nil
		}
		if sh.Mode == db.ShareModeSnapshot {
			if sh.SnapshotMsgID == nil {
				return false, nil
			}
			return database.AttachmentInSnapshot(sh.SessionID, c.Params("attachmentId"), *sh.SnapshotMsgID)
		}
		return true, nil
	}
	return false, nil
}
