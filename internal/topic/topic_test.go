package topic

import (
	"context"
	"testing"

	tb "gopkg.in/telebot.v3"
)

type fakeTelegramContext struct {
	tb.Context
	chat    *tb.Chat
	message *tb.Message
}

func (f fakeTelegramContext) Chat() *tb.Chat       { return f.chat }
func (f fakeTelegramContext) Message() *tb.Message { return f.message }

func TestForChat(t *testing.T) {
	m := &tb.Message{Chat: &tb.Chat{ID: -1001, Type: tb.ChatSuperGroup}, TopicMessage: true, ThreadID: 55}
	c := fakeTelegramContext{chat: m.Chat, message: m}
	if got := ThreadID(ForChat(c, -1001)); got != 55 {
		t.Fatalf("same-chat thread = %d, want 55", got)
	}
	if got := ThreadID(ForChat(c, -2002)); got != 0 {
		t.Fatalf("cross-chat topic leaked into channel: %d", got)
	}
	if got := ThreadID(Context(c)); got != 55 {
		t.Fatalf("Context topic = %d, want 55", got)
	}
}

func TestFromMessage(t *testing.T) {
	for _, tt := range []struct {
		name string
		m    *tb.Message
		want int
	}{
		{"nil", nil, 0},
		{"regular private chat", &tb.Message{Chat: &tb.Chat{Type: tb.ChatPrivate}, ThreadID: 99, TopicMessage: true}, 0},
		{"regular supergroup", &tb.Message{Chat: &tb.Chat{Type: tb.ChatSuperGroup}, ThreadID: 17}, 0},
		{"General topic", &tb.Message{Chat: &tb.Chat{Type: tb.ChatSuperGroup}, ThreadID: 1, TopicMessage: true}, 0},
		{"forum topic", &tb.Message{Chat: &tb.Chat{Type: tb.ChatSuperGroup}, ThreadID: 23, TopicMessage: true}, 23},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromMessage(tt.m); got != tt.want {
				t.Fatalf("FromMessage = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWithThreadID(t *testing.T) {
	ctx := WithThreadID(context.Background(), 123)
	if got := ThreadID(ctx); got != 123 {
		t.Fatalf("ThreadID = %d, want 123", got)
	}
	if got := ThreadID(context.Background()); got != 0 {
		t.Fatalf("unscoped ThreadID = %d, want 0", got)
	}
}
