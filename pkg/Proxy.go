package pkg

import (
	"log/slog"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/JackExperts/traefik-bluegreen/pkg/common"
	"github.com/JackExperts/traefik-bluegreen/pkg/redis"
)

type Proxy struct {
	ProxyURL  *url.URL
	RedisConn *redis.RedisStore
}

func (p *Proxy) RewriteProxy() func(*httputil.ProxyRequest) {

	return func(pr *httputil.ProxyRequest) {
		pr.SetURL(p.ProxyURL)
		pr.Out.Host = pr.In.Host

		tenant := common.VerifyEmpty(pr.In.URL.Query().Get("tenant"), "000000") // tenant default => 000000
		app := common.VerifyEmpty(pr.In.Header.Get("X-App-Slug"), "default")    // app default => default

		appPath := strings.Split(pr.In.URL.Path, "/")[1]

		slog.Info("Application Path", "path", appPath)

		tenantModel, err := p.RedisConn.GetSlot(tenant, app)
		slot := "-1" // Caso não encontre o valor no Redis nem no Cache

		if err != nil {
			slog.Error(err.Error())
		} else {
			slot = tenantModel.Slot
		}

		pr.Out.Header.Set("X-Slot", slot)

		pr.SetXForwarded()
	}
}
