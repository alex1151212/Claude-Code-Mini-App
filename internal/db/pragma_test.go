package db

import "testing"

// DSN 參數寫錯時 modernc 會靜默忽略；這裡直接驗證 pragma 真的生效。
func TestOpen_PragmasApplied(t *testing.T) {
	database, err := Open(t.TempDir() + "/pragma.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var jm string
	var bt int
	if err := database.QueryRow(`PRAGMA journal_mode`).Scan(&jm); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`PRAGMA busy_timeout`).Scan(&bt); err != nil {
		t.Fatal(err)
	}
	if jm != "wal" || bt != 5000 {
		t.Fatalf("journal_mode=%s busy_timeout=%d", jm, bt)
	}
}
