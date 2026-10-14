package feed

import (
	"net/url"
	"path"
	"strings"

	"github.com/indes/flowerss-bot/internal/model"
	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
)

// MediaItems extracts original media URLs in the order provided by a feed.
// We never fetch remote media: Telegram downloads each file from its URL.
// The cap prevents a malformed feed from flooding a chat with hundreds of files.
func MediaItems(item *gofeed.Item) []model.Media {
	if item == nil {
		return nil
	}
	const maxMedia = 40
	var results []model.Media
	seen := map[string]bool{}
	base, _ := url.Parse(item.Link)

	add := func(raw, mime string, fallback model.MediaKind) bool {
		if len(results) >= maxMedia {
			return false
		}
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || raw == "" {
			return false
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return false
		}
		u.Fragment = ""
		cleanURL := u.String()
		if seen[cleanURL] {
			return true
		}
		kind := mediaKind(u, mime, fallback)
		if kind == "" {
			return false
		}
		seen[cleanURL] = true
		results = append(results, model.Media{URL: cleanURL, Kind: kind})
		return true
	}

	// HTML content is scanned before enclosures because it preserves post order.
	// Video posters and Media RSS thumbnails are used only as a last resort.
	var fallbackImages []string
	for _, body := range []string{item.Content, item.Description} {
		z := html.NewTokenizer(strings.NewReader(body))
		inVideo, inAudio, inPicture := 0, 0, 0
		videoSourceChosen := false
		for len(results) < maxMedia {
			token := z.Next()
			if token == html.ErrorToken {
				break
			}
			if token == html.EndTagToken {
				tag, _ := z.TagName()
				switch string(tag) {
				case "video":
					if inVideo > 0 {
						inVideo--
					}
					videoSourceChosen = false
				case "audio":
					if inAudio > 0 {
						inAudio--
					}
				case "picture":
					if inPicture > 0 {
						inPicture--
					}
				}
				continue
			}
			if token != html.StartTagToken && token != html.SelfClosingTagToken {
				continue
			}
			tagBytes, hasAttrs := z.TagName()
			tag := string(tagBytes)
			if tag == "video" {
				inVideo++
			} else if tag == "audio" {
				inAudio++
			} else if tag == "picture" {
				inPicture++
			}
			if !hasAttrs {
				continue
			}
			attrs := map[string]string{}
			for {
				key, value, more := z.TagAttr()
				attrs[string(key)] = string(value)
				if !more {
					break
				}
			}
			switch tag {
			case "img":
				if isDecorativeImage(attrs) {
					continue
				}
				for _, candidate := range []string{attrs["data-src"], attrs["data-original"], attrs["src"], firstSrcset(attrs["srcset"])} {
					if add(candidate, "", model.MediaPhoto) {
						break
					}
				}
			case "video":
				videoSourceChosen = add(attrs["src"], attrs["type"], model.MediaVideo)
				if attrs["poster"] != "" {
					fallbackImages = append(fallbackImages, attrs["poster"])
				}
			case "audio":
				add(attrs["src"], attrs["type"], model.MediaAudio)
			case "source":
				switch {
				case inVideo > 0 && !videoSourceChosen:
					videoSourceChosen = add(attrs["src"], attrs["type"], model.MediaVideo)
					if !videoSourceChosen {
						videoSourceChosen = add(firstSrcset(attrs["srcset"]), attrs["type"], model.MediaVideo)
					}
				case inAudio > 0:
					add(attrs["src"], attrs["type"], model.MediaAudio)
				case inPicture > 0:
					// The <img> fallback already represents the responsive picture.
				}
			case "a":
				// Some RSSHub routes expose media through direct hyperlinks.
				add(attrs["href"], "", "")
			}
		}
	}

	for _, enclosure := range item.Enclosures {
		if enclosure != nil {
			add(enclosure.URL, enclosure.Type, "")
		}
	}
	for _, groupKind := range []string{"content", "group"} {
		for _, media := range item.Extensions["media"][groupKind] {
			if groupKind == "content" {
				add(media.Attrs["url"], media.Attrs["type"], mediumKind(media.Attrs["medium"]))
				continue
			}
			for _, nested := range media.Children["content"] {
				add(nested.Attrs["url"], nested.Attrs["type"], mediumKind(nested.Attrs["medium"]))
			}
		}
	}
	// A thumbnail does not become a separate image in a post that already has
	// real media (especially one that contains a playable video).
	if len(results) == 0 {
		for _, groupKind := range []string{"thumbnail", "group"} {
			for _, media := range item.Extensions["media"][groupKind] {
				if groupKind == "thumbnail" {
					add(media.Attrs["url"], media.Attrs["type"], model.MediaPhoto)
					continue
				}
				for _, nested := range media.Children["thumbnail"] {
					add(nested.Attrs["url"], nested.Attrs["type"], model.MediaPhoto)
				}
			}
		}
		if item.Image != nil {
			add(item.Image.URL, "", model.MediaPhoto)
		}
		for _, poster := range fallbackImages {
			add(poster, "", model.MediaPhoto)
		}
	}
	return results
}

func isDecorativeImage(attrs map[string]string) bool {
	className := strings.ToLower(attrs["class"])
	for _, part := range strings.Fields(className) {
		if strings.Contains(part, "emoji") || strings.Contains(part, "avatar") ||
			part == "icon" || part == "twemoji" {
			return true
		}
	}
	src := strings.ToLower(attrs["src"])
	return strings.Contains(src, "/profile_images/") || strings.Contains(src, "/emoji/")
}

func firstSrcset(srcset string) string {
	parts := strings.Fields(strings.SplitN(srcset, ",", 2)[0])
	if len(parts) != 0 {
		return parts[0]
	}
	return ""
}

func mediumKind(medium string) model.MediaKind {
	switch strings.ToLower(medium) {
	case "image":
		return model.MediaPhoto
	case "video":
		return model.MediaVideo
	case "audio":
		return model.MediaAudio
	default:
		return ""
	}
}

// mediaKind prefers explicit MIME, then a recognizable extension, then
// the HTML element's type. X's "tweet_video" MP4 files are silent animations.
func mediaKind(u *url.URL, mime string, fallback model.MediaKind) model.MediaKind {
	mime = strings.ToLower(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0]))
	lowerPath := strings.ToLower(u.Path)
	ext := strings.ToLower(path.Ext(lowerPath))
	queryFormat := strings.ToLower(u.Query().Get("format"))
	switch {
	case mime == "image/gif" || ext == ".gif" || queryFormat == "gif":
		return model.MediaAnimation
	case mime == "video/mp4" || ext == ".mp4" || queryFormat == "mp4":
		if strings.Contains(lowerPath, "/tweet_video/") || strings.Contains(lowerPath, "/animated_gif/") {
			return model.MediaAnimation
		}
		return model.MediaVideo
	case strings.HasPrefix(mime, "video/"):
		return model.MediaVideo
	case strings.HasPrefix(mime, "audio/") || ext == ".mp3" || ext == ".m4a" || ext == ".ogg" || ext == ".opus":
		return model.MediaAudio
	case strings.HasPrefix(mime, "image/") || ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" || ext == ".avif":
		return model.MediaPhoto
	}
	return fallback
}
