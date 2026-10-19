package bot

import (
	"fmt"

	"github.com/indes/flowerss-bot/internal/model"
	tb "gopkg.in/telebot.v3"
)

// sendNewsMedia delivers photos/videos as ordered Telegram albums (2-10 files),
// while animations/GIFs and audio travel as individual messages. The caption
// is attached to the first successfully sent media item or group.
func (b *Bot) sendNewsMedia(to tb.Recipient, opts *tb.SendOptions, media []model.Media, caption string) (bool, error) {
	if len(media) == 0 {
		return false, nil
	}
	if !albumCaptionFits(caption) {
		caption = ""
	}
	captionSent := false
	var sendError error

	sendSingle := func(item model.Media) {
		var what interface{}
		title := ""
		if !captionSent {
			title = caption
		}
		switch item.Kind {
		case model.MediaPhoto:
			what = &tb.Photo{File: tb.FromURL(item.URL), Caption: title}
		case model.MediaVideo:
			what = &tb.Video{File: tb.FromURL(item.URL), Caption: title, Streaming: true}
		case model.MediaAnimation:
			what = &tb.Animation{File: tb.FromURL(item.URL), Caption: title}
		case model.MediaAudio:
			what = &tb.Audio{File: tb.FromURL(item.URL), Caption: title}
		default:
			return
		}
		err := b.sendNewsMessage(to, what, opts)
		if err != nil && isMediaURLFetchError(err) {
			// Telegram cannot always fetch proxy URLs; upload server-side as a fallback.
			if local, fetchErr := downloadRSSMedia(item, title); fetchErr == nil {
				err = b.sendNewsMessage(to, local, opts)
			} else {
				err = fmt.Errorf("download media fallback: %w (Telegram: %v)", fetchErr, err)
			}
		}
		if err != nil {
			sendError = fmt.Errorf("send RSS media %s: %w", item.Kind, err)
		} else if title != "" {
			captionSent = true
		}
	}

	for i := 0; i < len(media); {
		item := media[i]
		if item.Kind != model.MediaPhoto && item.Kind != model.MediaVideo {
			sendSingle(item)
			i++
			continue
		}
		// Photo and video can share one album; GIF/animation and audio cannot.
		j := i
		for j < len(media) && j-i < 10 &&
			(media[j].Kind == model.MediaPhoto || media[j].Kind == model.MediaVideo) {
			j++
		}
		if j-i < 2 {
			sendSingle(item)
			i++
			continue
		}
		album := make(tb.Album, 0, j-i)
		for k := i; k < j; k++ {
			switch media[k].Kind {
			case model.MediaPhoto:
				album = append(album, &tb.Photo{File: tb.FromURL(media[k].URL)})
			case model.MediaVideo:
				album = append(album, &tb.Video{File: tb.FromURL(media[k].URL), Streaming: true})
			}
		}
		if !captionSent {
			album.SetCaption(caption)
		}
		if err := b.sendNewsAlbum(to, album, opts); err != nil {
			// A single broken image must not suppress every other media item.
			for k := i; k < j; k++ {
				sendSingle(media[k])
			}
		} else if caption != "" {
			captionSent = true
		}
		i = j
	}
	return captionSent, sendError
}
