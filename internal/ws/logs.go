package ws

import (
	fiberws "github.com/gofiber/contrib/websocket"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/logging"
)

// logsMsg 一次推多筆：連線後第一則是 backlog，其後是即時日誌（突發時合併成一則）。
type logsMsg struct {
	Entries []logging.Entry `json:"entries"`
}

// NewLogStreamHandler：只推不收的日誌 WS；讀迴圈僅用來偵測斷線。
// 這條連線本身不寫日誌，否則「連線 → 記一筆 → 推給自己」會自我放大。
func NewLogStreamHandler() func(*fiberws.Conn) {
	return func(c *fiberws.Conn) {
		backlog, live, unsub := logging.Subscribe()
		defer unsub()

		gone := make(chan struct{})
		go func() {
			defer close(gone)
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}()

		if c.WriteJSON(logsMsg{Entries: backlog}) != nil {
			return
		}
		for {
			select {
			case e := <-live:
				batch := []logging.Entry{e}
			drain:
				for len(batch) < 200 {
					select {
					case e := <-live:
						batch = append(batch, e)
					default:
						break drain
					}
				}
				if c.WriteJSON(logsMsg{Entries: batch}) != nil {
					return
				}
			case <-gone:
				return
			}
		}
	}
}
