package feed

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
)

func TestImageURLs(t *testing.T) {
	item := &gofeed.Item{
		Link:        "https://example.org/posts/42",
		Content:     `<img src="https://cdn.example.org/1.jpg"><img src="data:image/png;base64,abc" data-src="/2.png"><img src="https://cdn.example.org/1.jpg">`,
		Description: `<p><img srcset="https://cdn.example.org/3.jpg 1x, https://cdn.example.org/4.jpg 2x"></p>`,
		Enclosures: []*gofeed.Enclosure{
			{URL: "https://cdn.example.org/5.jpg", Type: "image/jpeg"},
			{URL: "https://cdn.example.org/video.mp4", Type: "video/mp4"},
		},
	}
	want := []string{
		"https://cdn.example.org/1.jpg", "https://example.org/2.png",
		"https://cdn.example.org/3.jpg", "https://cdn.example.org/5.jpg",
	}
	if got := ImageURLs(item); !reflect.DeepEqual(got, want) {
		t.Fatalf("ImageURLs = %q, want %q", got, want)
	}
}

func TestImageURLsLimitsAndInvalidSources(t *testing.T) {
	var html strings.Builder
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&html, `<img src="https://example.org/%d.jpg">`, i)
	}
	item := &gofeed.Item{Content: html.String() + `<img src="javascript:alert(1)">`}
	images := ImageURLs(item)
	if len(images) != 10 || images[9] != "https://example.org/9.jpg" {
		t.Fatalf("want first 10 photos, got %q", images)
	}
	if got := ImageURLs(nil); got != nil {
		t.Fatalf("nil item should produce nil, got %q", got)
	}
}

func TestImageURLsMediaRSS(t *testing.T) {
	item := &gofeed.Item{
		Extensions: ext.Extensions{
			"media": {
				"content": {{Attrs: map[string]string{"url": "https://cdn.example.org/photo1.jpg", "medium": "image"}}},
				"group": {{Children: map[string][]ext.Extension{
					"content":   {{Attrs: map[string]string{"url": "https://cdn.example.org/video.mp4", "type": "video/mp4"}}},
					"thumbnail": {{Attrs: map[string]string{"url": "https://cdn.example.org/photo2.jpg"}}},
				}}},
			},
		},
	}
	want := []string{"https://cdn.example.org/photo1.jpg", "https://cdn.example.org/photo2.jpg"}
	if got := ImageURLs(item); !reflect.DeepEqual(got, want) {
		t.Fatalf("Media RSS images = %q, want %q", got, want)
	}
}
