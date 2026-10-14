package bot

import tb "gopkg.in/telebot.v3"

// newsAlbum puts the RSS message in the first photo's caption when it fits.
// For longer messages, the caller sends the original text after the album.
func newsAlbum(urls []string, caption string) (tb.Album, bool) {
	if len(urls) < 2 {
		return nil, false
	}
	if len(urls) > 10 {
		urls = urls[:10]
	}

	captionFits := albumCaptionFits(caption)
	album := make(tb.Album, 0, len(urls))
	for i, imageURL := range urls {
		photo := &tb.Photo{File: tb.FromURL(imageURL)}
		if i == 0 && captionFits {
			photo.Caption = caption
		}
		album = append(album, photo)
	}
	return album, captionFits
}

// Telegram limits photo captions to 1024 UTF-16 characters. Counting the
// unparsed markup is conservative and avoids truncating HTML or Markdown tags.
func albumCaptionFits(caption string) bool {
	length := 0
	for _, r := range caption {
		length++
		if r > 0xffff {
			length++
		}
		if length > 1024 {
			return false
		}
	}
	return true
}
