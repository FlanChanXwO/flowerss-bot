package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/indes/flowerss-bot/internal/model"
	"github.com/mmcdole/gofeed"
)

// Opt-in integration test against the RSSHub instance supplied by the operator.
// Invoke with RSSHUB_ACCESS_KEY set in the environment; never check keys into Git.
func TestRSSHubMediaIntegration(t *testing.T) {
	key := os.Getenv("RSSHUB_ACCESS_KEY")
	if key == "" {
		t.Skip("RSSHUB_ACCESS_KEY not configured")
	}
	base := os.Getenv("RSSHUB_BASE_URL")
	if base == "" {
		base = "https://rss.atri.rodeo"
	}
	client := &http.Client{Timeout: 25 * time.Second}
	var photos, videos, animations int
	for _, route := range []string{"/twitter/media/NASA", "/twitter/media/SpaceX"} {
		u, err := url.Parse(base + route)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("key", key)
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("User-Agent", "Flowerss-RSS-media-compat-test")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("RSSHub %s fetch failed: %T", route, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Fatalf("RSSHub %s status: %d", route, resp.StatusCode)
		}
		feed, err := gofeed.NewParser().Parse(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("RSSHub %s feed parse: %v", route, err)
		}
		if len(feed.Items) == 0 {
			t.Fatalf("RSSHub %s returned empty feed", route)
		}
		for _, item := range feed.Items {
			for _, media := range MediaItems(item) {
				switch media.Kind {
				case model.MediaPhoto:
					photos++
				case model.MediaVideo:
					videos++
				case model.MediaAnimation:
					animations++
				}
			}
		}
		t.Logf("RSSHub route %s returned %d RSS items", route, len(feed.Items))
	}
	t.Log(fmt.Sprintf("Extracted %d photos, %d videos, %d animations", photos, videos, animations))
	if photos == 0 || videos == 0 {
		t.Fatalf("expected real image and video media, got photos=%d videos=%d", photos, videos)
	}
}
