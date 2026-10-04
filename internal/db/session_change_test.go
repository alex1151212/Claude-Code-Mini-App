package db

import "testing"

func TestUpdateSessionStatus_FiresOnSessionChange(t *testing.T) {
	database, err := Open(t.TempDir() + "/change.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s, err := database.CreateSession("t", "", "", "default", "claude", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	database.OnSessionChange = func() { n++ }
	if err := database.UpdateSessionStatus(s.ID, SessionStatusRunning); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("OnSessionChange 應被呼叫 1 次，實際 %d", n)
	}
}


// 啟動重設：running 與「沒有 pending_denials 的 awaiting_confirm」（kiroacp 中途授權，通道已隨重啟消失）改回 idle；
// Claude 的 awaiting_confirm 有 pending_denials，重啟後仍可用「允許此操作」重跑，必須保留。
func TestResetRunningSessions_ClearsStaleAwaitingConfirm(t *testing.T) {
	database, err := Open(t.TempDir() + "/reset.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mk := func(status, denials string) string {
		s, err := database.CreateSession("t", "", "", "default", "kiroacp", nil, "agent")
		if err != nil {
			t.Fatal(err)
		}
		_ = database.UpdateSessionStatus(s.ID, status)
		_ = database.UpdatePendingDenials(s.ID, denials)
		return s.ID
	}
	running := mk(SessionStatusRunning, "")
	stale := mk(SessionStatusAwaitingConfirm, "")
	claude := mk(SessionStatusAwaitingConfirm, `[{"tool_name":"Write"}]`)

	if err := database.ResetRunningSessions(); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{running: SessionStatusIdle, stale: SessionStatusIdle, claude: SessionStatusAwaitingConfirm} {
		s, _ := database.GetSession(id)
		if s.Status != want {
			t.Fatalf("session %s status=%s, want %s", id, s.Status, want)
		}
	}
}
