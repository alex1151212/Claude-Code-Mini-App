package mcp

import "testing"

func TestParseQueryTime_TimezoneToUTC(t *testing.T) {
	got, err := parseQueryTime("2026-09-09T00:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-09-08 16:00:00" {
		t.Fatalf("got %q", got)
	}
	got, err = parseQueryTime("2026-09-09 08:00:00")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-09-09 08:00:00" {
		t.Fatalf("無時區應視為 UTC，got %q", got)
	}
}

func TestNormPath(t *testing.T) {
	a := normPath(`C:\Users\user\Documents\EVE`)
	b := normPath(`C:/Users/user/Documents/EVE/`)
	if a != b || a != "c:/users/user/documents/eve" {
		t.Fatalf("a=%q b=%q", a, b)
	}
}

func TestTruncateRunes(t *testing.T) {
	got, trunc := truncateRunes("你好世界", 2)
	if !trunc || got != "你好…" {
		t.Fatalf("got %q trunc=%v", got, trunc)
	}
}
