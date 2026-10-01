package logging

import (
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	ringCap       = 2000 // 記憶體內保留的日誌筆數
	maxEntryBytes = 4096 // 單筆上限；agent 原始 JSON 動輒數 KB，完整內容仍在 logs/server.log
	subBuf        = 256  // 每個訂閱者的緩衝；滿了就丟，不能拖住整個行程的 logging
)

// Entry 是日誌頁看到的一筆日誌（Msg 可能含換行與結尾欄位）。
type Entry struct {
	TS    string `json:"ts"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// Telegram bot token 形如 123456789:AA...；Go 的 http 錯誤字串常把含 token 的 URL 帶出來。
var tgTokenRe = regexp.MustCompile(`\d{8,12}:[A-Za-z0-9_-]{30,}`)

// ring 是掛在 zap 上的 io.Writer：zap 對每筆日誌只呼叫一次 Write，且內容是已編碼完成的整筆（含多行），
// 所以不必自己解析檔案。Write 內不可再寫日誌，否則會遞迴。
type ring struct {
	mu     sync.Mutex
	buf    []Entry
	head   int // 下一個寫入位置
	n      int
	nextID uint64
	subs   map[uint64]chan Entry
}

func newRing(capacity int) *ring {
	return &ring{buf: make([]Entry, capacity), subs: make(map[uint64]chan Entry)}
}

var logRing = newRing(ringCap)

func (r *ring) Write(p []byte) (int, error) {
	e := parseEntry(string(p))
	r.mu.Lock()
	r.buf[r.head] = e
	r.head = (r.head + 1) % len(r.buf)
	if r.n < len(r.buf) {
		r.n++
	}
	for _, ch := range r.subs {
		select {
		case ch <- e:
		default: // ponytail: 慢的客戶端直接丟行，要求不掉行再改成每訂閱者獨立 goroutine＋佇列
		}
	}
	r.mu.Unlock()
	return len(p), nil
}

// subscribe 在同一把鎖內取 backlog 並註冊，避免兩者之間漏行或重複。
func (r *ring) subscribe() ([]Entry, <-chan Entry, func()) {
	ch := make(chan Entry, subBuf)
	r.mu.Lock()
	backlog := make([]Entry, 0, r.n)
	start := 0
	if r.n == len(r.buf) {
		start = r.head
	}
	for i := 0; i < r.n; i++ {
		backlog = append(backlog, r.buf[(start+i)%len(r.buf)])
	}
	r.nextID++
	id := r.nextID
	r.subs[id] = ch
	r.mu.Unlock()
	return backlog, ch, func() {
		r.mu.Lock()
		delete(r.subs, id)
		r.mu.Unlock()
	}
}

// Subscribe 回傳目前保留的日誌與之後的即時日誌；unsub 必須在連線結束時呼叫。
func Subscribe() (backlog []Entry, live <-chan Entry, unsub func()) {
	return logRing.subscribe()
}

// parseEntry 解析 zap console encoder 的輸出：「時間\t等級\t訊息[\t欄位]」。
func parseEntry(line string) Entry {
	line = strings.TrimRight(line, "\r\n")
	e := Entry{Level: "INFO", Msg: line}
	if parts := strings.SplitN(line, "\t", 3); len(parts) == 3 {
		e = Entry{TS: parts[0], Level: parts[1], Msg: parts[2]}
	}
	e.Msg = tgTokenRe.ReplaceAllString(e.Msg, "<redacted-bot-token>")
	if len(e.Msg) > maxEntryBytes {
		cut := maxEntryBytes
		for cut > 0 && !utf8.RuneStart(e.Msg[cut]) {
			cut--
		}
		e.Msg = e.Msg[:cut] + "…（已截斷，完整內容見 logs/server.log）"
	}
	return e
}
