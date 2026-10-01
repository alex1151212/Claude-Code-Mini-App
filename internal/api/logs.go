package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/logging"
)

// GetLogLevel GET /logs/level → {"debug": bool}
func GetLogLevel(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"debug": logging.DebugEnabled()})
}

// PutLogLevel PUT /logs/level，body {"debug": bool}：執行期開關 Debug 日誌（重啟後回到 LOG_LEVEL）。
func PutLogLevel(c *fiber.Ctx) error {
	var body struct {
		Debug *bool `json:"debug"`
	}
	if err := c.BodyParser(&body); err != nil || body.Debug == nil {
		return jsonErr(c, 400, "需要 debug 布林欄位")
	}
	logging.SetDebug(*body.Debug)
	slog.Info("[logs] Debug 日誌切換", "debug", *body.Debug)
	return c.JSON(fiber.Map{"debug": logging.DebugEnabled()})
}
