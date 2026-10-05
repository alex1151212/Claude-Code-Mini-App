package ws

import (
	"sort"
	"sync"
)

// ownerDisplayName 是擁有者在在線名單與署名中的顯示名稱（messages.author 空字串）。
const ownerDisplayName = "擁有者"

// 結束分享的原因（share_ended 事件的 content）。
const (
	shareEndRevoked = "revoked"
	shareEndExpired = "expired"
)

// connEntry 是一條 session WS 連線在「在線名單」中的登記。
// shareID == 0 代表擁有者連線；否則為該分享的訪客連線。
type connEntry struct {
	sessionID string
	shareID   int64
	nickname  string // 訪客暱稱；擁有者為空
	kick      func(reason string)
}

func (e *connEntry) displayName() string {
	if e.shareID == 0 {
		return ownerDisplayName
	}
	return e.nickname
}

var presenceReg = struct {
	mu      sync.Mutex
	nextID  uint64
	entries map[uint64]*connEntry
}{entries: map[uint64]*connEntry{}}

// presenceAdd 登記連線，回傳取消登記的函式。
func presenceAdd(e *connEntry) (remove func()) {
	presenceReg.mu.Lock()
	presenceReg.nextID++
	id := presenceReg.nextID
	presenceReg.entries[id] = e
	presenceReg.mu.Unlock()
	return func() {
		presenceReg.mu.Lock()
		delete(presenceReg.entries, id)
		presenceReg.mu.Unlock()
	}
}

// presenceNames 回傳該 session 目前在線的顯示名稱（去重；擁有者排第一，其餘依字母序）。
func presenceNames(sessionID string) []string {
	presenceReg.mu.Lock()
	defer presenceReg.mu.Unlock()
	seen := map[string]bool{}
	owner := false
	var names []string
	for _, e := range presenceReg.entries {
		if e.sessionID != sessionID {
			continue
		}
		if e.shareID == 0 {
			owner = true
			continue
		}
		if !seen[e.nickname] {
			seen[e.nickname] = true
			names = append(names, e.nickname)
		}
	}
	sort.Strings(names)
	if owner {
		names = append([]string{ownerDisplayName}, names...)
	}
	return names
}

// sessionHasGuests 回報該 session 是否有訪客在線。
func sessionHasGuests(sessionID string) bool {
	presenceReg.mu.Lock()
	defer presenceReg.mu.Unlock()
	for _, e := range presenceReg.entries {
		if e.sessionID == sessionID && e.shareID != 0 {
			return true
		}
	}
	return false
}

// ShareOnline 回傳某分享目前在線的訪客暱稱（去重）。
func ShareOnline(shareID int64) []string {
	presenceReg.mu.Lock()
	defer presenceReg.mu.Unlock()
	seen := map[string]bool{}
	names := []string{}
	for _, e := range presenceReg.entries {
		if e.shareID == shareID && !seen[e.nickname] {
			seen[e.nickname] = true
			names = append(names, e.nickname)
		}
	}
	sort.Strings(names)
	return names
}

// KickShare 通知並關閉某分享的所有訪客 WS（分享被結束時呼叫）。
func KickShare(shareID int64) {
	presenceReg.mu.Lock()
	var targets []*connEntry
	for _, e := range presenceReg.entries {
		if e.shareID == shareID {
			targets = append(targets, e)
		}
	}
	presenceReg.mu.Unlock()
	for _, e := range targets {
		e.kick(shareEndRevoked)
	}
}

// announcePresence 廣播在線名單。只在「有訪客參與」時才廣播，沒分享過的聊天室不多出任何訊息。
func announcePresence(sessionID, event, who string, force bool) {
	if !force && !sessionHasGuests(sessionID) {
		return
	}
	hub.Broadcast(sessionID, serverMsg{
		Type:   "presence",
		Value:  event,
		Author: who,
		Online: presenceNames(sessionID),
	})
}
