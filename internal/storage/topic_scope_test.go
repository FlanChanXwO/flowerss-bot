package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/indes/flowerss-bot/internal/model"
	"github.com/indes/flowerss-bot/internal/topic"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTopicSubscriptionsMigrateAndStayIsolated(t *testing.T) {
	// Simulate an existing deployment before it knew about Telegram topics.
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE subscribes (id integer primary key, user_id integer, source_id integer, enable_notification integer)").Error; err != nil {
		t.Fatal(err)
	}
	const chatID int64 = -100123456
	if err := db.Exec("INSERT INTO subscribes (id,user_id,source_id,enable_notification) VALUES (1,?,7,1)", chatID).Error; err != nil {
		t.Fatal(err)
	}
	store := NewSubscriptionStorageImpl(db)
	general := context.Background()
	thread11 := topic.WithThreadID(general, 11)
	thread22 := topic.WithThreadID(general, 22)
	if err := store.Init(general); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	existing, err := store.GetSubscription(general, chatID, 7)
	if err != nil || existing.ThreadID != 0 || existing.Paused {
		t.Fatalf("legacy subscription lost: %#v, %v", existing, err)
	}
	for _, thread := range []struct {
		ctx context.Context
		id  int
	}{
		{thread11, 11}, {thread22, 22},
	} {
		exists, err := store.SubscriptionExist(thread.ctx, chatID, 7)
		if err != nil || exists {
			t.Fatalf("unexpected duplicate for thread %d: %t, %v", thread.id, exists, err)
		}
		if err := store.AddSubscription(thread.ctx, &model.Subscribe{UserID: chatID, ThreadID: thread.id, SourceID: 7, EnableNotification: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if subscriptions, err := store.GetSubscriptionsBySourceID(general, 7, &GetSubscriptionsOptions{Count: -1}); err != nil || len(subscriptions.Subscriptions) != 3 {
		t.Fatalf("expected same RSS for 3 topics: %#v, %v", subscriptions, err)
	}
	for _, thread := range []context.Context{general, thread11, thread22} {
		result, err := store.GetSubscriptionsByUserID(thread, chatID, &GetSubscriptionsOptions{Count: -1})
		if err != nil || len(result.Subscriptions) != 1 || result.Subscriptions[0].ThreadID != topic.ThreadID(thread) {
			t.Fatalf("wrong topic list for %d: %#v, %v", topic.ThreadID(thread), result, err)
		}
	}
	s11, err := store.GetSubscription(thread11, chatID, 7)
	if err != nil {
		t.Fatal(err)
	}
	s11.Paused = true
	if err := store.UpsertSubscription(thread11, chatID, 7, s11); err != nil {
		t.Fatal(err)
	}
	s22, err := store.GetSubscription(thread22, chatID, 7)
	if err != nil || s22.Paused {
		t.Fatalf("pausing thread 11 affected thread 22: %#v, %v", s22, err)
	}
	if n, err := store.DeleteSubscription(thread11, chatID, 7); err != nil || n != 1 {
		t.Fatalf("thread 11 unsubscribe failed: %d, %v", n, err)
	}
	if _, err := store.GetSubscription(thread11, chatID, 7); err == nil {
		t.Fatal("unsubscribed topic still visible")
	}
	for _, thread := range []context.Context{general, thread22} {
		if _, err := store.GetSubscription(thread, chatID, 7); err != nil {
			t.Fatalf("unsubscribing topic 11 affected topic %d: %v", topic.ThreadID(thread), err)
		}
	}
	if count, err := store.CountSourceSubscriptions(general, 7); err != nil || count != 2 {
		t.Fatalf("feed source incorrectly orphaned: count=%d err=%v", count, err)
	}
	// Applying the migration again must not destroy existing subscriptions.
	if err := store.Init(general); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSubscription(general, chatID, 7); err != nil {
		t.Fatalf("migration is not idempotent: %v", err)
	}
}
