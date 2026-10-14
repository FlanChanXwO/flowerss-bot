package feed

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/indes/flowerss-bot/internal/model"
	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
)

func TestMediaItemsHTML(t *testing.T) {
	item := &gofeed.Item{
		Link: "https://example.org/post/42",
		Content: `<p><img src="https://cdn.example.org/1.jpg"><img src="https://cdn.example.org/1.jpg">
<video poster="https://cdn.example.org/poster.png"><source src="/clip.mp4" type="video/mp4"></video>
<img src="https://cdn.example.org/gif.gif"><audio><source src="https://cdn.example.org/music.mp3" type="audio/mpeg"></audio></p>`,
	}
	want := []model.Media{
		{URL: "https://cdn.example.org/1.jpg", Kind: model.MediaPhoto},
		{URL: "https://example.org/clip.mp4", Kind: model.MediaVideo},
		{URL: "https://cdn.example.org/gif.gif", Kind: model.MediaAnimation},
		{URL: "https://cdn.example.org/music.mp3", Kind: model.MediaAudio},
	}
	if got := MediaItems(item); !reflect.DeepEqual(got, want) {
		t.Fatalf("MediaItems = %#v, want %#v", got, want)
	}
}

func TestMediaItemsXAnimatedGIF(t *testing.T) {
	item := &gofeed.Item{Content: `<video src="https://video.twimg.com/tweet_video/abc123.mp4"></video>
<a href="https://video.twimg.com/ext_tw_video/def456/pu/vid/720x1280/clip.mp4">video</a>
<img src="https://pbs.twimg.com/media/abc?format=gif&amp;name=large">
<img data-src="//pbs.twimg.com/media/photo?format=jpg&amp;name=large" src="data:image/png;base64,xyz">`}
	want := []model.Media{
		{URL: "https://video.twimg.com/tweet_video/abc123.mp4", Kind: model.MediaAnimation},
		{URL: "https://video.twimg.com/ext_tw_video/def456/pu/vid/720x1280/clip.mp4", Kind: model.MediaVideo},
		{URL: "https://pbs.twimg.com/media/abc?format=gif&name=large", Kind: model.MediaAnimation},
	}
	// The protocol-relative URL cannot be resolved without a base URL; it is
	// permitted only when the item link provides the source document's host.
	item.Link = "https://twitter.com/example/status/123"
	want = append(want, model.Media{URL: "https://pbs.twimg.com/media/photo?format=jpg&name=large", Kind: model.MediaPhoto})
	if got := MediaItems(item); !reflect.DeepEqual(got, want) {
		t.Fatalf("X animation parsing = %#v, want %#v", got, want)
	}
}

func TestMediaItemsEnclosureAndMediaRSS(t *testing.T) {
	item := &gofeed.Item{
		Link: "https://example.com/1",
		Enclosures: []*gofeed.Enclosure{
			{URL: "https://example.com/clip", Type: "video/mp4"},
			{URL: "https://example.com/gif", Type: "image/gif"},
			{URL: "https://example.com/track", Type: "audio/mpeg"},
		},
		Extensions: ext.Extensions{"media": {
			"group": {{Children: map[string][]ext.Extension{
				"content":   {{Attrs: map[string]string{"url": "https://example.com/photo.jpg", "medium": "image"}}},
				"thumbnail": {{Attrs: map[string]string{"url": "https://example.com/poster.png"}}},
			}}},
		}},
	}
	want := []model.Media{
		{URL: "https://example.com/clip", Kind: model.MediaVideo},
		{URL: "https://example.com/gif", Kind: model.MediaAnimation},
		{URL: "https://example.com/track", Kind: model.MediaAudio},
		{URL: "https://example.com/photo.jpg", Kind: model.MediaPhoto},
	}
	if got := MediaItems(item); !reflect.DeepEqual(got, want) {
		t.Fatalf("Media RSS = %#v, want %#v", got, want)
	}
}

func TestMediaItemsFallbackAndLimits(t *testing.T) {
	item := &gofeed.Item{Content: `<video poster="https://cdn.example.com/poster.jpg"></video>`}
	if got := MediaItems(item); !reflect.DeepEqual(got, []model.Media{{URL: "https://cdn.example.com/poster.jpg", Kind: model.MediaPhoto}}) {
		t.Fatalf("video poster fallback = %#v", got)
	}
	var body strings.Builder
	for i := 0; i < 45; i++ {
		fmt.Fprintf(&body, `<img src="https://cdn.example.com/%d.jpg">`, i)
	}
	body.WriteString(`<img src="javascript:alert(1)"><img src="data:image/png;base64,zzz">`)
	items := MediaItems(&gofeed.Item{Content: body.String()})
	if len(items) != 40 || items[39].URL != "https://cdn.example.com/39.jpg" {
		t.Fatalf("media count capped at 40: %#v", items)
	}
	if MediaItems(nil) != nil {
		t.Fatal("nil item should produce nil media")
	}
}

func TestMediaItemsDoNotDuplicateResponsivePicturesAndVideoVariants(t *testing.T) {
	item := &gofeed.Item{Content: `<img class="emoji" src="https://example.com/smile.png">
<picture><source srcset="https://example.com/photo-2x.jpg 2x">
<img src="https://example.com/photo.jpg"></picture>
<video><source src="https://example.com/video.mp4" type="video/mp4">
<source src="https://example.com/video.webm" type="video/webm"></video>`}
	want := []model.Media{
		{URL: "https://example.com/photo.jpg", Kind: model.MediaPhoto},
		{URL: "https://example.com/video.mp4", Kind: model.MediaVideo},
	}
	if got := MediaItems(item); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected one image and one video, got %#v", got)
	}
}
