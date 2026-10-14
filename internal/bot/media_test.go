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

	"github.com/indes/flowerss-bot/internal/config"
	"github.com/indes/flowerss-bot/internal/model"
	tb "gopkg.in/telebot.v3"
)

type mediaAPICall struct {
	Method  string
	Caption string
	Media   []map[string]interface{}
}

func TestBroadcastRichMedia(t *testing.T) {
	base := []model.Media{
		{URL: "https://example.com/photo.jpg", Kind: model.MediaPhoto},
		{URL: "https://example.com/video.mp4", Kind: model.MediaVideo},
	}
	for _, tc := range []struct {
		name          string
		media         []model.Media
		title         string
		description   string
		telegraphURL  string
		enableTG      uint
		previewLength int
		errorMethod   string
		want          []string
		wantCaption   bool
	}{
		{name: "single photo", media: base[:1], title: "hello", want: []string{"sendPhoto"}, wantCaption: true},
		{name: "single video", media: base[1:], title: "hello", want: []string{"sendVideo"}, wantCaption: true},
		{name: "animated GIF", media: []model.Media{{URL: "https://example.com/clip.gif", Kind: model.MediaAnimation}}, title: "hello", want: []string{"sendAnimation"}, wantCaption: true},
		{name: "audio", media: []model.Media{{URL: "https://example.com/audio.mp3", Kind: model.MediaAudio}}, title: "hello", want: []string{"sendAudio"}, wantCaption: true},
		{name: "mixed photo and video", media: base, title: "hello", want: []string{"sendMediaGroup"}, wantCaption: true},
		{name: "animation then mixed", media: append([]model.Media{{URL: "https://example.com/clip.gif", Kind: model.MediaAnimation}}, base...), title: "hello", want: []string{"sendAnimation", "sendMediaGroup"}, wantCaption: true},
		{name: "mixed then audio", media: append(append([]model.Media{}, base...), model.Media{URL: "https://example.com/audio.mp3", Kind: model.MediaAudio}), title: "hello", want: []string{"sendMediaGroup", "sendAudio"}, wantCaption: true},
		{name: "long body without telegraph", media: base, title: "hello", description: strings.Repeat("X", 1350), previewLength: 1400, want: []string{"sendMediaGroup", "sendMessage"}},
		{name: "long body with telegraph", media: base, title: "hello", description: strings.Repeat("X", 1350), previewLength: 1400, telegraphURL: "https://telegra.ph/read-01-01", enableTG: 1, want: []string{"sendMediaGroup"}, wantCaption: true},
		{name: "unavailable video", media: base[1:], title: "hello", errorMethod: "sendVideo", want: []string{"sendVideo", "sendMessage"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := config.PreviewText
			config.PreviewText = tc.previewLength
			defer func() { config.PreviewText = prev }()
			var calls []mediaAPICall
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				params := map[string]string{}
				if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
					t.Errorf("invalid request: %v", err)
					return
				}
				if got := params["message_thread_id"]; got != "99" {
					t.Errorf("%s posted to thread=%q, want 99", r.URL.Path, got)
				}
				method := strings.TrimPrefix(r.URL.Path, "/botTEST/")
				c := mediaAPICall{Method: method, Caption: params["caption"]}
				if method == "sendMediaGroup" {
					if err := json.Unmarshal([]byte(params["media"]), &c.Media); err != nil {
						t.Errorf("invalid group: %v", err)
					}
					if len(c.Media) > 0 {
						c.Caption, _ = c.Media[0]["caption"].(string)
					}
				}
				calls = append(calls, c)
				if method == tc.errorMethod {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"Bad Request: no valid media URL"}`)
					return
				}
				switch method {
				case "sendMediaGroup":
					fmt.Fprint(w, `{"ok":true,"result":[]}`)
				case "sendPhoto":
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"photo":[{"file_id":"photo1"}]}}`)
				case "sendVideo":
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"video":{"file_id":"video1"}}}`)
				case "sendAnimation":
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"animation":{"file_id":"anim1"}}}`)
				case "sendAudio":
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"audio":{"file_id":"audio1"}}}`)
				case "sendMessage":
					fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
				default:
					t.Errorf("unexpected request %s", method)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer api.Close()
			tg, err := tb.NewBot(tb.Settings{URL: api.URL, Token: "TEST", Client: api.Client(), Offline: true})
			if err != nil {
				t.Fatal(err)
			}
			b := &Bot{tb: tg}
			b.BroadcastNews(
				&model.Source{ID: 1, Title: "RSS", Link: "https://example.com/feed"},
				[]*model.Subscribe{{UserID: -10012, ThreadID: 99, EnableTelegraph: int(tc.enableTG)}},
				[]*model.Content{{Title: tc.title, Description: tc.description, RawLink: "https://example.com/post", TelegraphURL: tc.telegraphURL, Media: tc.media}},
			)
			var names []string
			for _, c := range calls {
				names = append(names, c.Method)
			}
			if !reflect.DeepEqual(names, tc.want) {
				t.Fatalf("API calls = %v, want %v", names, tc.want)
			}
			if tc.wantCaption && len(calls) > 0 && calls[0].Caption == "" {
				t.Fatalf("media caption missing: %#v", calls)
			}
			if tc.telegraphURL != "" && !strings.Contains(calls[0].Caption, tc.telegraphURL) {
				t.Fatalf("long Telegraph article lost reading link: %#v", calls)
			}
			if tc.name == "mixed photo and video" && (calls[0].Media[0]["type"] != "photo" || calls[0].Media[1]["type"] != "video") {
				t.Fatalf("wrong media types: %#v", calls[0].Media)
			}
		})
	}
}

func TestAlbumBatchesOverTen(t *testing.T) {
	media := make([]model.Media, 23)
	for i := range media {
		media[i] = model.Media{URL: fmt.Sprintf("https://example.com/%d.jpg", i), Kind: model.MediaPhoto}
	}
	var groupCounts []int
	var captions []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params map[string]string
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Error(err)
			return
		}
		if params["message_thread_id"] != strconv.Itoa(77) {
			t.Errorf("missing forum topic in album")
		}
		var group []map[string]interface{}
		if err := json.Unmarshal([]byte(params["media"]), &group); err != nil {
			t.Error(err)
		}
		groupCounts = append(groupCounts, len(group))
		captions = append(captions, fmt.Sprint(group[0]["caption"]))
		fmt.Fprint(w, `{"ok":true,"result":[]}`)
	}))
	defer api.Close()
	tg, err := tb.NewBot(tb.Settings{URL: api.URL, Token: "TEST", Client: api.Client(), Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	b := &Bot{tb: tg}
	captioned, err := b.sendNewsMedia(&tb.Chat{ID: 123}, &tb.SendOptions{ThreadID: 77}, media, "A short caption")
	if err != nil || !captioned {
		t.Fatalf("send media: captioned=%v error=%v", captioned, err)
	}
	if !reflect.DeepEqual(groupCounts, []int{10, 10, 3}) {
		t.Fatalf("wrong batch lengths: %v", groupCounts)
	}
	if captions[0] != "A short caption" || captions[1] != "" || captions[2] != "" {
		t.Fatalf("caption must only appear once: %v", captions)
	}
}
