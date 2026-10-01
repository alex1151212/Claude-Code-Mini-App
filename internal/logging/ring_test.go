package logging

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func line(msg string) []byte { return []byte("2026-10-01T10:00:00.000+0800\tINFO\t" + msg + "\n") }

func msgs(es []Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Msg
	}
	return out
}

func TestRingKeepsNewestInOrder(t *testing.T) {
	r := newRing(3)
	for i := 1; i <= 5; i++ {
		r.Write(line(fmt.Sprintf("m%d", i)))
	}
	backlog, _, unsub := r.subscribe()
	defer unsub()
	if got := strings.Join(msgs(backlog), ","); got != "m3,m4,m5" {
		t.Fatalf("backlog = %s, want m3,m4,m5", got)
	}
}

func TestParseEntryKeepsMultilineAndFields(t *testing.T) {
	e := parseEntry("2026-10-01T10:00:00.000+0800\tWARN\t第一行\n第二行\t{\"k\":1}\n")
	if e.Level != "WARN" || e.TS == "" || e.Msg != "第一行\n第二行\t{\"k\":1}" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if got := parseEntry("沒有分隔的一行\n"); got.Level != "INFO" || got.Msg != "沒有分隔的一行" {
		t.Fatalf("fallback entry: %+v", got)
	}
}

func TestParseEntryTruncatesOnRuneBoundary(t *testing.T) {
	e := parseEntry("t\tINFO\t" + strings.Repeat("中", 3000)) // 9000 bytes
	if !utf8.ValidString(e.Msg) {
		t.Fatal("截斷切壞了 UTF-8")
	}
	if len(e.Msg) > maxEntryBytes+200 {
		t.Fatalf("未截斷：%d bytes", len(e.Msg))
	}
}

func TestParseEntryRedactsBotToken(t *testing.T) {
	tok := "123456789:" + strings.Repeat("A", 35)
	e := parseEntry("t\tERROR\tPost https://api.telegram.org/bot" + tok + "/sendMessage failed")
	if strings.Contains(e.Msg, tok) || !strings.Contains(e.Msg, "<redacted-bot-token>") {
		t.Fatalf("token 未遮蔽：%s", e.Msg)
	}
}

func TestSlowSubscriberDoesNotBlockWriter(t *testing.T) {
	r := newRing(10)
	_, ch, unsub := r.subscribe()
	defer unsub()
	for i := 0; i < subBuf+50; i++ { // 沒人讀 ch，Write 不能卡住
		r.Write(line("x"))
	}
	if len(ch) != subBuf {
		t.Fatalf("len(ch) = %d, want %d（超出的應被丟棄）", len(ch), subBuf)
	}
}

func TestSubscribeReceivesLiveAfterBacklog(t *testing.T) {
	r := newRing(5)
	r.Write(line("old"))
	backlog, ch, unsub := r.subscribe()
	defer unsub()
	r.Write(line("new"))
	if len(backlog) != 1 || backlog[0].Msg != "old" {
		t.Fatalf("backlog = %v", msgs(backlog))
	}
	if e := <-ch; e.Msg != "new" {
		t.Fatalf("live = %q, want new", e.Msg)
	}
}
