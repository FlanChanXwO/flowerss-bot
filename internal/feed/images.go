package feed

import (
	"net/url"
	"strings"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
)

// ImageURLs returns up to ten distinct photos from an RSS item, in feed order.
// It accepts HTML images, image enclosures and Media RSS image attachments.
func ImageURLs(item *gofeed.Item) []string {
	if item == nil {
		return nil
	}
	const limit = 10 // Telegram sendMediaGroup accepts at most ten media items.
	var images []string
	seen := make(map[string]bool)
	base, _ := url.Parse(item.Link)

	add := func(raw string) bool {
		raw = strings.TrimSpace(raw)
		if raw == "" || len(images) == limit {
			return false
		}
		u, err := url.Parse(raw)
		if err != nil {
			return false
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return false
		}
		u.Fragment = ""
		imageURL := u.String()
		if !seen[imageURL] {
			seen[imageURL] = true
			images = append(images, imageURL)
		}
		return true
	}

	for _, body := range []string{item.Content, item.Description} {
		z := html.NewTokenizer(strings.NewReader(body))
		for len(images) < limit {
			token := z.Next()
			if token == html.ErrorToken {
				break
			}
			if token != html.StartTagToken && token != html.SelfClosingTagToken {
				continue
			}
			tag, hasAttrs := z.TagName()
			if string(tag) != "img" || !hasAttrs {
				continue
			}
			attrs := make(map[string]string)
			for {
				key, val, more := z.TagAttr()
				attrs[string(key)] = string(val)
				if !more {
					break
				}
			}
			srcset := strings.Fields(strings.SplitN(attrs["srcset"], ",", 2)[0])
			var fromSrcset string
			if len(srcset) > 0 {
				fromSrcset = srcset[0]
			}
			for _, candidate := range []string{attrs["data-src"], attrs["data-original"], attrs["src"], fromSrcset} {
				if add(candidate) {
					break
				}
			}
		}
	}
	if item.Image != nil {
		add(item.Image.URL)
	}
	for _, enclosure := range item.Enclosures {
		if enclosure != nil && strings.HasPrefix(strings.ToLower(enclosure.Type), "image/") {
			add(enclosure.URL)
		}
	}
	addMedia := func(attrs map[string]string) {
		if strings.EqualFold(attrs["medium"], "video") {
			return
		}
		if typ := attrs["type"]; typ != "" && !strings.HasPrefix(strings.ToLower(typ), "image/") {
			return
		}
		add(attrs["url"])
	}
	for _, kind := range []string{"content", "thumbnail"} {
		for _, media := range item.Extensions["media"][kind] {
			addMedia(media.Attrs)
		}
		for _, group := range item.Extensions["media"]["group"] {
			for _, media := range group.Children[kind] {
				addMedia(media.Attrs)
			}
		}
	}
	return images
}
