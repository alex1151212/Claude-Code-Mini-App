package ws

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	fiberws "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/logging"
)

// readEntries 讀一則 WS 訊息並回傳所有 msg 串接（逾時就失敗）。
func readEntries(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var m logsMsg
	if err := conn.ReadJSON(&m); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, e := range m.Entries {
		sb.WriteString(e.Msg + "\n")
	}
	return sb.String()
}

func TestLogStream_BacklogThenLive(t *testing.T) {
	t.Chdir(t.TempDir()) // logging.Init 會在目前目錄建 logs/
	prev := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(prev)
		// SetDefault 還原預設 handler 時不會重設標準庫 log 的輸出，不還原的話之後的 log.Print 會經 zap 在別的目錄重開 server.log
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	t.Cleanup(logging.Init()) // 先關檔再清暫存目錄，否則 Windows 上刪不掉 server.log

	slog.Info("backlog-line")

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/logs/ws", fiberws.New(NewLogStreamHandler()))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	url := fmt.Sprintf("ws://127.0.0.1:%d/logs/ws", ln.Addr().(*net.TCPAddr).Port)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if got := readEntries(t, conn); !strings.Contains(got, "backlog-line") {
		t.Fatalf("第一則應含 backlog，got %q", got)
	}
	slog.Info("live-line")
	if got := readEntries(t, conn); !strings.Contains(got, "live-line") {
		t.Fatalf("應即時收到新日誌，got %q", got)
	}
}
