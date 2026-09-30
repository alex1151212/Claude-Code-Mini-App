package ws

import (
	"context"
	"sync"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/proc"
)

type taskEntry struct {
	cancel   context.CancelFunc
	msgID    int64
	pid      int  // 子進程 PID，Run() 啟動後由 taskSetPid 補上
	softSent bool // 是否已送過優雅停止信號（第一次按停止）
}

var taskManager struct {
	mu    sync.Mutex
	tasks map[string]*taskEntry
}

func init() {
	taskManager.tasks = make(map[string]*taskEntry)
}

// dispatchLocks 序列化「判斷忙碌→排隊或啟動」與「佇列取出→啟動」。
// runAgent 在 taskCancel 與 taskStart 之間有 DB I/O 空窗；不加鎖時新 input 可能在空窗判定成閒置而插隊。
// 以 session 為 key（同一 session 可能有多條 WS 連線，各自持有 closure）。
// ponytail: 刪除 session 後 mutex 不回收；每個 session 一個，量小可忽略，真的累積再於 DeleteSession 時清掉。
var dispatchLocks sync.Map // sessionID → *sync.Mutex

func dispatchLock(sessionID string) *sync.Mutex {
	m, _ := dispatchLocks.LoadOrStore(sessionID, &sync.Mutex{})
	return m.(*sync.Mutex)
}

func taskStart(sessionID string, cancel context.CancelFunc, msgID int64) {
	taskManager.mu.Lock()
	if old, ok := taskManager.tasks[sessionID]; ok && old.cancel != nil {
		old.cancel()
	}
	taskManager.tasks[sessionID] = &taskEntry{cancel: cancel, msgID: msgID}
	taskManager.mu.Unlock()
}

// taskSetPid 記錄任務對應子進程的 PID，供 taskInterrupt 第一階段送優雅停止信號用。
// 僅在目前登記的任務仍是呼叫者自己（msgID 相同）時才寫入，避免舊任務污染新任務。
func taskSetPid(sessionID string, msgID int64, pid int) {
	taskManager.mu.Lock()
	if e, ok := taskManager.tasks[sessionID]; ok && e.msgID == msgID {
		e.pid = pid
	}
	taskManager.mu.Unlock()
}

// taskEnd 只在目前登記的任務仍是呼叫者自己（msgID 相同）時才刪除，
// 避免舊任務（例如權限被拒後已被 taskCancel 取代）的延遲收尾把新任務的登記誤刪。
func taskEnd(sessionID string, msgID int64) {
	taskManager.mu.Lock()
	if e, ok := taskManager.tasks[sessionID]; ok && e.msgID == msgID {
		delete(taskManager.tasks, sessionID)
	}
	taskManager.mu.Unlock()
}

func taskIsActive(sessionID string) bool {
	taskManager.mu.Lock()
	defer taskManager.mu.Unlock()
	_, ok := taskManager.tasks[sessionID]
	return ok
}

// taskCancel 立即強制終止任務（cancel context → cmd.Cancel 觸發 KillTree）。
// 供 reset_context、rerun 等需要立即清乾淨、不留優雅緩衝的路徑使用。
func taskCancel(sessionID string) {
	taskManager.mu.Lock()
	e, ok := taskManager.tasks[sessionID]
	if !ok {
		taskManager.mu.Unlock()
		return
	}
	delete(taskManager.tasks, sessionID)
	taskManager.mu.Unlock()
	if e != nil && e.cancel != nil {
		e.cancel()
	}
}

// taskInterrupt 實作兩段式「停止」：
//   - 任務仍在跑、且尚未送過優雅信號、且 PID 已知 → 只送 SendInterrupt（可能被子進程忽略），
//     不 cancel context、不動 DB／狀態，任務繼續跑，等使用者自己判斷要不要再按一次。
//   - 已經按過一次（softSent）、或 PID 還沒拿到 → 直接 taskCancel 強制終止（既有的乾淨收尾路徑）。
//
// 回傳 true 表示有動作被執行（任務仍在跑），false 表示目前沒有任務可停。
func taskInterrupt(sessionID string) bool {
	taskManager.mu.Lock()
	e, ok := taskManager.tasks[sessionID]
	if !ok {
		taskManager.mu.Unlock()
		return false
	}
	if !e.softSent && e.pid != 0 {
		e.softSent = true
		pid := e.pid
		taskManager.mu.Unlock()
		_ = proc.SendInterrupt(pid)
		return true
	}
	taskManager.mu.Unlock()
	taskCancel(sessionID)
	return true
}
