package bot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/indes/flowerss-bot/internal/model"
	tb "gopkg.in/telebot.v3"
)

func TestBroadcastNewsAlbum(t *testing.T) {
	photos := []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"}
	for _, tc := range []struct {
		name        string
		images      []string
		title       string
		albumError  bool
		wantCalls   []string
		wantCaption bool
		threadID    int
		paused      bool
	}{
		{name: "album caption", images: photos, title: "Short title", wantCalls: []string{"sendMediaGroup"}, wantCaption: true},
		{name: "long text", images: photos, title: strings.Repeat("a", 1100), wantCalls: []string{"sendMediaGroup", "sendMessage"}},
		{name: "failed album", images: photos, title: "Short title", albumError: true, wantCalls: []string{"sendMediaGroup", "sendPhoto", "sendPhoto"}, wantCaption: true},
		{name: "text only", title: "Short title", wantCalls: []string{"sendMessage"}},
		{name: "single photo", images: photos[:1], title: "Short title", wantCalls: []string{"sendPhoto"}},
		{name: "forum album", images: photos, title: "Short title", threadID: 123, wantCalls: []string{"sendMediaGroup"}, wantCaption: true},
		{name: "forum long text", images: photos, title: strings.Repeat("b", 1100), threadID: 123, wantCalls: []string{"sendMediaGroup", "sendMessage"}},
		{name: "forum text", title: "Short title", threadID: 123, wantCalls: []string{"sendMessage"}},
		{name: "forum paused subscription", images: photos, title: "Short title", threadID: 123, paused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			var media []map[string]string
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var params map[string]string
				if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
					t.Errorf("invalid Bot API request: %v", err)
					return
				}
				if got := params["message_thread_id"]; got != "" || tc.threadID != 0 {
					if got != strconv.Itoa(tc.threadID) {
						t.Errorf("message_thread_id = %q, expected %d", got, tc.threadID)
					}
				}
				switch r.URL.Path {
				case "/botTEST/sendMediaGroup":
					calls = append(calls, "sendMediaGroup")
					if err := json.Unmarshal([]byte(params["media"]), &media); err != nil {
						t.Errorf("invalid media group: %v", err)
					}
					if tc.albumError {
						w.WriteHeader(http.StatusBadRequest)
						fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"Bad Request: image unavailable"}`)
						return
					}
					fmt.Fprint(w, `{"ok":true,"result":[]}`)
				case "/botTEST/sendPhoto":
					calls = append(calls, "sendPhoto")
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"photo":[{"file_id":"photo-1"}]}}`)
				case "/botTEST/sendMessage":
					calls = append(calls, "sendMessage")
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
				default:
					t.Errorf("unexpected API method: %s", r.URL.Path)
				}
			}))
			defer api.Close()

			tg, err := tb.NewBot(tb.Settings{URL: api.URL, Token: "TEST", Client: api.Client(), Offline: true})
			if err != nil {
				t.Fatal(err)
			}
			b := &Bot{tb: tg}
			b.BroadcastNews(
				&model.Source{ID: 1, Title: "Feed", Link: "https://example.com/rss"},
				[]*model.Subscribe{{UserID: 42, ThreadID: tc.threadID, Paused: tc.paused}},
				[]*model.Content{{Title: tc.title, RawLink: "https://example.com/post", ImageURLs: tc.images}},
			)
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("API calls = %v, want %v", calls, tc.wantCalls)
			}
			if len(tc.images) >= 2 && !tc.paused {
				if len(media) != 2 || media[0]["media"] != photos[0] || media[1]["media"] != photos[1] {
					t.Fatalf("wrong album contents: %v", media)
				}
				if (media[0]["caption"] != "") != tc.wantCaption {
					t.Fatalf("album caption incorrect: %v", media[0]["caption"])
				}
			}
		})
	}
}
