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
