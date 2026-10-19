package bot

import (
	"errors"
	"time"

	"github.com/indes/flowerss-bot/internal/log"
	tb "gopkg.in/telebot.v3"
)

// sendNewsMessage and sendNewsAlbum share one rate limiter for RSS delivery.
func (b *Bot) sendNewsMessage(to tb.Recipient, message interface{}, opts *tb.SendOptions) error {
	return b.sendWithRateLimit(to, 1, func() error {
		_, err := b.tb.Send(to, message, opts)
		return err
	})
}

func (b *Bot) sendNewsAlbum(to tb.Recipient, album tb.Album, opts *tb.SendOptions) error {
	return b.sendWithRateLimit(to, len(album), func() error {
		_, err := b.tb.SendAlbum(to, album, opts)
		return err
	})
}

// Serialize RSS sends and space messages per destination chat. A Telegram
// media group can contain several messages, so reserve one slot per item.
// On 429, honor retry_after instead of discarding an already-stored RSS item.
func (b *Bot) sendWithRateLimit(to tb.Recipient, messages int, send func() error) error {
	b.sendMu.Lock()
	defer b.sendMu.Unlock()

	if b.nextSend == nil {
		b.nextSend = make(map[string]time.Time)
	}
	chat := to.Recipient()
	for {
		if delay := time.Until(b.nextSend[chat]); delay > 0 {
			time.Sleep(delay)
		}

		err := send()
		b.nextSend[chat] = time.Now().Add(time.Duration(messages) * b.sendInterval)
		if err == nil {
			return nil
		}

		var flood tb.FloodError
		if !errors.As(err, &flood) {
			return err
		}
		delay := time.Duration(flood.RetryAfter+1) * time.Second
		if delay < time.Second {
			delay = time.Second
		}
		b.nextSend[chat] = time.Now().Add(delay)
		log.Warnf("Telegram rate limited RSS delivery; retrying chat=%s after %s", chat, delay)
	}
}
