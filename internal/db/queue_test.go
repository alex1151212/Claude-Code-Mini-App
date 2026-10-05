package db

import "testing"

func TestQueue_FIFOPauseAndDelete(t *testing.T) {
	database, err := Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s, err := database.CreateSession("t", "", "/tmp/q", "default", "claude", nil, "agent")
	if err != nil {
		t.Fatal(err)
	}

	a, _ := database.EnqueueMessage(s.ID, "a", "")
	b, _ := database.EnqueueMessage(s.ID, "b", "")
	if _, err := database.EnqueueMessage(s.ID, "c", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteQueuedMessage("other-session", b.ID); err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteQueuedMessage(s.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := database.ListQueuedMessages(s.ID)
	if len(list) != 2 || list[0].ID != a.ID || list[1].Content != "c" {
		t.Fatalf("list=%+v", list)
	}

	if err := database.PauseQueueIfPending(s.ID); err != nil {
		t.Fatal(err)
	}
	if p, _ := database.QueuePaused(s.ID); !p {
		t.Fatal("有待執行訊息時應暫停")
	}

	for _, want := range []string{"a", "c"} {
		q, err := database.PopQueuedMessage(s.ID)
		if err != nil || q == nil || q.Content != want {
			t.Fatalf("pop want %q got %+v err=%v", want, q, err)
		}
	}
	if q, _ := database.PopQueuedMessage(s.ID); q != nil {
		t.Fatalf("空佇列應回 nil，got %+v", q)
	}

	// 空佇列不暫停；新一批入列時解除上一批留下的暫停。
	_ = database.SetQueuePaused(s.ID, false)
	_ = database.PauseQueueIfPending(s.ID)
	if p, _ := database.QueuePaused(s.ID); p {
		t.Fatal("空佇列不應暫停")
	}
	_ = database.SetQueuePaused(s.ID, true)
	_, _ = database.EnqueueMessage(s.ID, "new batch", "")
	if p, _ := database.QueuePaused(s.ID); p {
		t.Fatal("空佇列入列應解除暫停")
	}

	if err := database.DeleteSession(s.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := database.ListQueuedMessages(s.ID); len(list) != 0 {
		t.Fatalf("刪 session 應清佇列，got %+v", list)
	}
}
