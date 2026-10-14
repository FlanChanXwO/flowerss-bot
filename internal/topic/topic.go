// Package topic scopes RSS subscriptions to a Telegram forum thread while
// preserving thread 0 for existing private, group, channel and General subscriptions.
package topic

import (
	"context"

	tb "gopkg.in/telebot.v3"
)

type contextKey struct{}

// FromMessage returns zero for a non-forum chat or the General topic.
// Telegram uses message_thread_id=1 for General; existing subscriptions use 0.
func FromMessage(m *tb.Message) int {
	if m == nil || m.Chat == nil || m.Chat.Type != tb.ChatSuperGroup || !m.TopicMessage || m.ThreadID <= 1 {
		return 0
	}
	return m.ThreadID
}

func WithThreadID(ctx context.Context, id int) context.Context {
	if id <= 0 {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, id)
}

func ThreadID(ctx context.Context) int {
	if id, ok := ctx.Value(contextKey{}).(int); ok {
		return id
	}
	return 0
}

// ForChat applies the current topic only when managing that same chat.
// A /sub @channel or /export @channel command must target the channel, not
// the forum topic from which the command was issued.
func ForChat(c tb.Context, chatID int64) context.Context {
	base := context.Background()
	if c == nil || c.Chat() == nil || c.Chat().ID != chatID {
		return base
	}
	return WithThreadID(base, FromMessage(c.Message()))
}

// Context applies the current Telegram message's forum topic if available.
func Context(c tb.Context) context.Context {
	if c == nil || c.Chat() == nil {
		return context.Background()
	}
	return ForChat(c, c.Chat().ID)
}

// Send keeps command output in the invoking topic. A reply preserves the
// original message's topic without modifying unrelated send options.
func Send(c tb.Context, what interface{}, opts ...interface{}) error {
	if FromMessage(c.Message()) != 0 && c.Callback() == nil {
		return c.Reply(what, opts...)
	}
	return c.Send(what, opts...)
}
