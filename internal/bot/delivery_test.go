package bot

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tb "gopkg.in/telebot.v3"
)

func newTestDeliveryBot(t *testing.T, server *httptest.Server) *Bot {
	t.Helper()
	tg, err := tb.NewBot(tb.Settings{URL: server.URL, Token: "TEST", Client: server.Client(), Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	return &Bot{tb: tg}
}

func TestRSSDeliveryRetriesTelegramFlood(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	}))
	defer server.Close()

	b := newTestDeliveryBot(t, server)
	start := time.Now()
	if err := b.sendNewsMessage(&tb.Chat{ID: -10042}, "test", &tb.SendOptions{ThreadID: 7}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if elapsed := time.Since(start); elapsed < 2*time.Second {
		t.Fatalf("retry-after ignored: elapsed %s", elapsed)
	}
}

func TestRSSDeliveryDoesNotRetryNonFloodErrors(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"Bad Request: failed to get HTTP URL content"}`)
	}))
	defer server.Close()

	b := newTestDeliveryBot(t, server)
	if err := b.sendNewsMessage(&tb.Chat{ID: -10042}, "test", nil); err == nil {
		t.Fatal("expected HTTP 400 error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRSSDeliveryRateLimitsSameChat(t *testing.T) {
	var mu sync.Mutex
	var timestamps []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	}))
	defer server.Close()

	b := newTestDeliveryBot(t, server)
	b.sendInterval = 70 * time.Millisecond
	for i := 0; i < 2; i++ {
		if err := b.sendNewsMessage(&tb.Chat{ID: -10042}, "test", nil); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(timestamps) != 2 {
		t.Fatalf("calls = %d, want 2", len(timestamps))
	}
	if gap := timestamps[1].Sub(timestamps[0]); gap < 60*time.Millisecond {
		t.Fatalf("same-chat send gap = %s, want at least 60ms", gap)
	}
}

func TestRSSDeliveryAlbumReservesAllMediaSlots(t *testing.T) {
	var mu sync.Mutex
	var timestamps []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/sendMediaGroup") {
			fmt.Fprint(w, `{"ok":true,"result":[]}`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	}))
	defer server.Close()

	b := newTestDeliveryBot(t, server)
	b.sendInterval = 30 * time.Millisecond
	album := tb.Album{
		&tb.Photo{File: tb.FromURL("https://example.com/1.jpg")},
		&tb.Photo{File: tb.FromURL("https://example.com/2.jpg")},
		&tb.Photo{File: tb.FromURL("https://example.com/3.jpg")},
	}
	if err := b.sendNewsAlbum(&tb.Chat{ID: -10042}, album, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.sendNewsMessage(&tb.Chat{ID: -10042}, "after album", nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(timestamps) != 2 {
		t.Fatalf("calls = %d, want 2", len(timestamps))
	}
	if gap := timestamps[1].Sub(timestamps[0]); gap < 80*time.Millisecond {
		t.Fatalf("album pacing gap = %s, want at least 80ms", gap)
	}
}
