package handler

import (
	"fmt"

	tb "gopkg.in/telebot.v3"

	"github.com/indes/flowerss-bot/internal/bot/session"
	"github.com/indes/flowerss-bot/internal/core"
	"github.com/indes/flowerss-bot/internal/topic"
)

type PauseAll struct {
	core *core.Core
}

func NewPauseAll(core *core.Core) *PauseAll {
	return &PauseAll{core: core}
}

func (p *PauseAll) Command() string {
	return "/pauseall"
}

func (p *PauseAll) Description() string {
	return "停止抓取所有订阅更新"
}

func (p *PauseAll) Handle(ctx tb.Context) error {
	subscribeUserID := ctx.Message().Chat.ID
	var channelChat *tb.Chat
	v := ctx.Get(session.StoreKeyMentionChat.String())
	if v != nil {
		var ok bool
		channelChat, ok = v.(*tb.Chat)
		if ok && channelChat != nil {
			subscribeUserID = channelChat.ID
		}
	}

	source, err := p.core.GetUserSubscribedSources(topic.ForChat(ctx, subscribeUserID), subscribeUserID)
	if err != nil {
		return ctx.Reply("系统错误")
	}

	for _, s := range source {
		err := p.core.SetSubscriptionPaused(topic.ForChat(ctx, subscribeUserID), subscribeUserID, s.ID, true)
		if err != nil {
			return ctx.Reply("暂停失败")
		}
	}

	reply := "订阅已全部暂停"
	if channelChat != nil {
		reply = fmt.Sprintf("频道 [%s](https://t.me/%s) 订阅已全部暂停", channelChat.Title, channelChat.Username)
	}
	return topic.Send(ctx,
		reply, &tb.SendOptions{
			DisableWebPagePreview: true,
			ParseMode:             tb.ModeMarkdown,
		},
	)
}

func (p *PauseAll) Middlewares() []tb.MiddlewareFunc {
	return nil
}
