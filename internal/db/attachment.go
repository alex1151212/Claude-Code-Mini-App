package db

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Attachment exposes display metadata; storage names never leave the server.
type Attachment struct {
	ID          string `json:"id"`
	SessionID   string `json:"-"`
	Name        string `json:"name"`
	StorageName string `json:"-"`
	MimeType    string `json:"mime_type"`
	Size        int64  `json:"size"`
	CreatedAt   string `json:"created_at"`
	URL         string `json:"url"`
}

func (db *DB) AddAttachment(a *Attachment) error {
	_, err := db.Exec(`INSERT INTO attachments (id, session_id, original_name, storage_name, mime_type, size) VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.SessionID, a.Name, a.StorageName, a.MimeType, a.Size)
	return err
}

func (db *DB) AttachmentMap(sessionID string) (map[string]Attachment, error) {
	rows, err := db.Query(`SELECT id, original_name, storage_name, mime_type, size, created_at FROM attachments WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]Attachment)
	for rows.Next() {
		a := Attachment{SessionID: sessionID}
		if err := rows.Scan(&a.ID, &a.Name, &a.StorageName, &a.MimeType, &a.Size, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.URL = "/sessions/" + url.PathEscape(sessionID) + "/uploads/" + url.PathEscape(a.ID)
		out[a.ID] = a
	}
	return out, rows.Err()
}

func (db *DB) GetAttachment(sessionID, id string) (*Attachment, error) {
	all, err := db.AttachmentMap(sessionID)
	if err != nil {
		return nil, err
	}
	a, ok := all[id]
	if !ok {
		return nil, fmt.Errorf("附件不存在或不屬於此會話")
	}
	return &a, nil
}

// ResolveAttachments preserves order and rejects unknown or duplicate IDs.
func (db *DB) ResolveAttachments(sessionID string, ids []string) ([]Attachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > 32 {
		return nil, fmt.Errorf("每則訊息最多 32 個附件")
	}
	all, err := db.AttachmentMap(sessionID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	out := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		a, ok := all[id]
		if !ok || seen[id] {
			return nil, fmt.Errorf("附件不存在、重複或不屬於此會話")
		}
		seen[id] = true
		out = append(out, a)
	}
	return out, nil
}

func attachmentIDsJSON(ids []string) string {
	if ids == nil {
		ids = []string{}
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

func decodeAttachmentIDs(raw string) ([]string, error) {
	var ids []string
	err := json.Unmarshal([]byte(raw), &ids)
	return ids, err
}

func hydrateAttachments(ids []string, all map[string]Attachment) []Attachment {
	out := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		if a, ok := all[id]; ok {
			out = append(out, a)
		}
	}
	return out
}

func (db *DB) AddUserMessageWithAttachments(sessionID, content string, ids []string) (*Message, error) {
	return db.saveUserMessage(sessionID, content, ids, 0)
}

// PromoteQueuedMessage moves a queue item into history in one transaction.
func (db *DB) PromoteQueuedMessage(sessionID string, q QueuedMessage) (*Message, error) {
	return db.saveUserMessage(sessionID, q.Content, q.AttachmentIDs, q.ID)
}

func (db *DB) saveUserMessage(sessionID, content string, ids []string, queueID int64) (*Message, error) {
	attachments, err := db.ResolveAttachments(sessionID, ids)
	if err != nil {
		return nil, err
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if queueID != 0 {
		res, err := tx.Exec(`DELETE FROM queued_messages WHERE session_id = ? AND id = ? AND content = ? AND attachment_ids = ?`, sessionID, queueID, content, attachmentIDsJSON(ids))
		if err != nil {
			return nil, err
		}
		n, err := res.RowsAffected()
		if err != nil || n != 1 {
			return nil, fmt.Errorf("排隊訊息已變更或不存在")
		}
	}
	res, err := tx.Exec(`INSERT INTO messages (session_id, role, content, status, attachment_ids) VALUES (?, 'user', ?, ?, ?)`,
		sessionID, content, MessageStatusDone, attachmentIDsJSON(ids))
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	m := &Message{ID: id, SessionID: sessionID, Role: "user", Content: content, Status: MessageStatusDone, AttachmentIDs: ids, Attachments: attachments}
	if err := tx.QueryRow(`SELECT created_at FROM messages WHERE id = ?`, id).Scan(&m.CreatedAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return m, nil
}
