package tgraph

import (
	"github.com/indes/flowerss-bot/internal/config"
	"github.com/indes/flowerss-bot/internal/log"

	"github.com/indes/telegraph-go"
)

var (
	socks5Proxy = config.Socks5
	clientPool  []*telegraph.Client
)

func init() {
	if !config.EnableTelegraph {
		return
	}
	for _, token := range config.TelegraphToken {
		client, err := telegraph.Load(token, socks5Proxy)
		if err != nil {
			// Never print Telegraph access tokens to log files.
			log.Warnf("load Telegraph account failed: %v", err)
			continue
		}
		clientPool = append(clientPool, client)
	}
	if len(clientPool) == 0 && config.TelegraphAccountName != "" {
		client, err := telegraph.Create(
			config.TelegraphAccountName,
			config.TelegraphAuthorName,
			config.TelegraphAuthorURL,
			config.Socks5,
		)
		if err != nil {
			log.Errorf("create Telegraph account failed: %v", err)
		} else {
			clientPool = append(clientPool, client)
			log.Info("created Telegraph account")
		}
	}
	if len(clientPool) == 0 {
		config.EnableTelegraph = false
		log.Warn("Telegraph unavailable; using original article links instead")
	} else {
		log.Infof("Telegraph enabled with %d account(s)", len(clientPool))
	}
}
