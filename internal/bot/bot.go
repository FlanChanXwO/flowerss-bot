package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	tb "gopkg.in/telebot.v3"

	"github.com/indes/flowerss-bot/internal/bot/handler"
	"github.com/indes/flowerss-bot/internal/bot/middleware"
	"github.com/indes/flowerss-bot/internal/bot/preview"
	"github.com/indes/flowerss-bot/internal/config"
	"github.com/indes/flowerss-bot/internal/core"
	"github.com/indes/flowerss-bot/internal/log"
	"github.com/indes/flowerss-bot/internal/model"
	"github.com/indes/flowerss-bot/internal/topic"
)

type Bot struct {
	core *core.Core
	tb           *tb.Bot // telebot.Bot instance
	sendMu       sync.Mutex
	nextSend     map[string]time.Time
	sendInterval time.Duration
}

func NewBot(core *core.Core) *Bot {
	log.Infof("init telegram bot, endpoint %s", config.TelegramEndpoint)
	settings := tb.Settings{
		URL:    config.TelegramEndpoint,
		Token:  config.BotToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		Client: core.HttpClient().Client(),
	}

	logLevel := config.GetString("log.level")
	if strings.ToLower(logLevel) == "debug" {
		settings.Verbose = true
	}

	b := &Bot{
		core:         core,
		sendInterval: 3500 * time.Millisecond, // Telegram supergroup limit: ~20 messages/minute.
	}

	var err error
	b.tb, err = tb.NewBot(settings)
	if err != nil {
		log.Error(err)
		return nil
	}
	b.tb.Use(middleware.UserFilter(), middleware.PreLoadMentionChat(), middleware.IsChatAdmin())
	return b
}

func (b *Bot) registerCommands(appCore *core.Core) error {
	commandHandlers := []handler.CommandHandler{
		handler.NewStart(),
		handler.NewPing(b.tb),
		handler.NewAddSubscription(appCore),
		handler.NewRemoveSubscription(b.tb, appCore),
		handler.NewListSubscription(appCore),
		handler.NewRemoveAllSubscription(),
		handler.NewOnDocument(b.tb, appCore),
		handler.NewSet(b.tb, appCore),
		handler.NewSetFeedTag(appCore),
		handler.NewSetUpdateInterval(appCore),
		handler.NewExport(appCore),
		handler.NewImport(),
		handler.NewPauseAll(appCore),
		handler.NewActiveAll(appCore),
		handler.NewHelp(),
		handler.NewVersion(),
	}

	for _, h := range commandHandlers {
		b.tb.Handle(h.Command(), h.Handle, h.Middlewares()...)
	}

	ButtonHandlers := []handler.ButtonHandler{
		handler.NewRemoveAllSubscriptionButton(appCore),
		handler.NewCancelRemoveAllSubscriptionButton(),
		handler.NewSetFeedItemButton(b.tb, appCore),
		handler.NewRemoveSubscriptionItemButton(appCore),
		handler.NewNotificationSwitchButton(b.tb, appCore),
		handler.NewSetSubscriptionTagButton(b.tb),
		handler.NewTelegraphSwitchButton(b.tb, appCore),
		handler.NewSubscriptionSwitchButton(b.tb, appCore),
	}

	for _, h := range ButtonHandlers {
		b.tb.Handle(h, h.Handle, h.Middlewares()...)
	}

	var commands []tb.Command
	for _, h := range commandHandlers {
		if h.Description() == "" {
			continue
		}
		commands = append(commands, tb.Command{Text: h.Command(), Description: h.Description()})
	}
	log.Debugf("set bot command %+v", commands)
	if err := b.tb.SetCommands(commands); err != nil {
		return err
	}
	return nil
}

func (b *Bot) Run() error {
	if config.RunMode == config.TestMode {
		return nil
	}

	if err := b.registerCommands(b.core); err != nil {
		return err
	}
	log.Infof("bot start %s", config.AppVersionInfo())
	b.tb.Start()
	return nil
}

func (b *Bot) SourceUpdate(
	source *model.Source, newContents []*model.Content, subscribes []*model.Subscribe,
) {
	b.BroadcastNews(source, subscribes, newContents)
}

func (b *Bot) SourceUpdateError(source *model.Source) {
	b.BroadcastSourceError(source)
}

// BroadcastNews send new contents message to subscriber
func (b *Bot) BroadcastNews(source *model.Source, subs []*model.Subscribe, contents []*model.Content) {
	zap.S().Infow(
		"broadcast news",
		"fetcher id", source.ID,
		"fetcher title", source.Title,
		"subscriber count", len(subs),
		"new contents", len(contents),
	)

	for _, content := range contents {
		previewText := preview.TrimDescription(content.Description, config.PreviewText)

		for _, sub := range subs {
			if sub.Paused {
				continue
			}
			tpldata := &config.TplData{
				SourceTitle:     source.Title,
				ContentTitle:    content.Title,
				RawLink:         content.RawLink,
				PreviewText:     previewText,
				TelegraphURL:    content.TelegraphURL,
				Tags:            sub.Tag,
				EnableTelegraph: sub.EnableTelegraph == 1 && content.TelegraphURL != "",
			}

			u := &tb.User{
				ID: sub.UserID,
			}
			o := &tb.SendOptions{
				DisableWebPagePreview: config.DisableWebPagePreview,
				ParseMode:             config.MessageMode,
				DisableNotification:   sub.EnableNotification != 1,
				ThreadID:              sub.ThreadID,
			}
			msg, err := tpldata.Render(config.MessageMode)
			if err != nil {
				zap.S().Errorw(
					"broadcast news error, tpldata.Render err",
					"error", err.Error(),
				)
				return
			}
			media := content.Media
			// Preserve the old in-memory content format for callers that construct
			// Content directly, rather than through the RSS parser.
			if len(media) == 0 {
				for _, url := range content.ImageURLs {
					media = append(media, model.Media{URL: url, Kind: model.MediaPhoto})
				}
			}
			if len(media) > 10 && tpldata.EnableTelegraph {
				// Keep large galleries in Telegraph when a reading page is available.
				media = nil
			}
			caption := msg
			if !albumCaptionFits(caption) && tpldata.EnableTelegraph {
				// For a long article, use a short media caption linking to its
				// Telegraph reading view rather than duplicating its whole preview.
				brief := *tpldata
				brief.PreviewText = ""
				if short, err := brief.Render(config.MessageMode); err == nil && albumCaptionFits(short) {
					caption = short
				}
			}
			captionSent, mediaErr := b.sendNewsMedia(u, o, media, caption)
			if mediaErr != nil {
				zap.S().Warnw("RSS media delivery was incomplete; original post link remains available",
					"error", mediaErr, "user id", sub.UserID, "source id", sub.SourceID)
			}
			if captionSent {
				continue
			}
			if err := b.sendNewsMessage(u, msg, o); err != nil {
				zap.S().Errorw("broadcast news text send failed", "error", err, "source id", source.ID, "user id", sub.UserID)

				if strings.Contains(err.Error(), "Forbidden") {
					zap.S().Errorw(
						"broadcast news error, bot stopped by user",
						"error", err.Error(),
						"user id", sub.UserID,
						"source id", sub.SourceID,
						"title", source.Title,
						"link", source.Link,
					)
					b.core.Unsubscribe(topic.WithThreadID(context.Background(), sub.ThreadID), sub.UserID, sub.SourceID)
				}

				/*
					Telegram return error if markdown message has incomplete format.
					Print the msg to warn the user
					api error: Bad Request: can't parse entities: Can't find end of the entity starting at byte offset 894
				*/
				if strings.Contains(err.Error(), "parse entities") {
					zap.S().Errorw(
						"broadcast news error, markdown error",
						"markdown msg", msg,
						"error", err.Error(),
					)
				}
			}
		}
	}
}

// BroadcastSourceError send fetcher update error message to subscribers
func (b *Bot) BroadcastSourceError(source *model.Source) {
	subs, err := b.core.GetSourceAllSubscriptions(context.Background(), source.ID)
	if err != nil {
		log.Errorf("get subscriptions failed, %v", err)
	}
	var u tb.User
	for _, sub := range subs {
		message := fmt.Sprintf(
			"[%s](%s) 已经连续%d次更新失败，将自动继续重试", source.Title, source.Link, config.ErrorThreshold,
		)
		u.ID = sub.UserID
		if err := b.sendNewsMessage(&u, message, &tb.SendOptions{
			ParseMode: tb.ModeMarkdown,
			ThreadID:  sub.ThreadID,
		}); err != nil {
			log.Errorf("notify source error failed: source=%d, err=%v", source.ID, err)
		}
	}
}
