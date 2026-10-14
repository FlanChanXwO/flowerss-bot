package core

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/indes/flowerss-bot/internal/storage"
	"github.com/indes/flowerss-bot/internal/topic"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestForumSubscriptionsAreIndependent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())))
	if err != nil {
		t.Fatal(err)
	}
	c := NewCore(
		storage.NewUserStorageImpl(db),
		storage.NewContentStorageImpl(db),
		storage.NewSourceStorageImpl(db),
		storage.NewSubscriptionStorageImpl(db),
		nil, nil,
	)
	if err := c.Init(); err != nil {
		t.Fatal(err)
	}
	const chatID int64 = -10014222
	const sourceID uint = 8
	general := context.Background()
	topic11 := topic.WithThreadID(general, 11)
	topic22 := topic.WithThreadID(general, 22)
	for _, ctx := range []context.Context{general, topic11, topic22} {
		if err := c.AddSubscription(ctx, chatID, sourceID); err != nil {
			t.Fatalf("subscribe thread %d: %v", topic.ThreadID(ctx), err)
		}
	}
	if err := c.AddSubscription(topic11, chatID, sourceID); !errors.Is(err, ErrSubscriptionExist) {
		t.Fatalf("duplicate subscription error = %v", err)
	}
	if err := c.SetSubscriptionPaused(topic11, chatID, sourceID, true); err != nil {
		t.Fatal(err)
	}
	if old, err := c.GetSubscription(topic11, chatID, sourceID); err != nil || !old.Paused {
		t.Fatalf("topic 11 was not paused: %#v, %v", old, err)
	}
	for _, ctx := range []context.Context{general, topic22} {
		other, err := c.GetSubscription(ctx, chatID, sourceID)
		if err != nil || other.Paused {
			t.Fatalf("pause leaked to topic %d: %#v, %v", topic.ThreadID(ctx), other, err)
		}
	}
	if err := c.Unsubscribe(topic11, chatID, sourceID); err != nil {
		t.Fatal(err)
	}
	subs, err := c.GetSourceAllSubscriptions(general, sourceID)
	if err != nil || len(subs) != 2 {
		t.Fatalf("deleting one topic removed others: %#v, %v", subs, err)
	}
	if err := c.Unsubscribe(topic11, chatID, sourceID); !errors.Is(err, ErrSubscriptionNotExist) {
		t.Fatalf("already-unsubscribed topic error = %v", err)
	}
	if _, err := c.GetSubscription(topic22, chatID, sourceID); err != nil {
		t.Fatalf("topic 22 subscription lost: %v", err)
	}
}
