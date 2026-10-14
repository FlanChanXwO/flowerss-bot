package bot

import (
	"strings"
	"testing"
)

func TestNewsAlbum(t *testing.T) {
	if album, captioned := newsAlbum([]string{"https://example.com/1.jpg"}, "hello"); len(album) != 0 || captioned {
		t.Fatal("a single image must not generate an album")
	}
	urls := make([]string, 12)
	for i := range urls {
		urls[i] = "https://example.com/photo.jpg"
	}
	album, captioned := newsAlbum(urls, "hello")
	if !captioned || len(album) != 10 {
		t.Fatalf("want 10 photos with caption, got %d captioned=%v", len(album), captioned)
	}
	if album[0].InputMedia().Caption != "hello" || album[1].InputMedia().Caption != "" {
		t.Fatal("only the first photo should carry the caption")
	}

	album, captioned = newsAlbum(urls, strings.Repeat("x", 1025))
	if captioned || album[0].InputMedia().Caption != "" {
		t.Fatal("long messages must be sent separately")
	}
	if albumCaptionFits(strings.Repeat("😀", 513)) {
		t.Fatal("emoji caption exceeds the UTF-16 limit")
	}
}
