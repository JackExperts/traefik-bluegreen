package traefik_bluegreen

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/PHBueno/traefik-bluegreen/pkg"
	"github.com/PHBueno/traefik-bluegreen/pkg/common"
	"github.com/PHBueno/traefik-bluegreen/pkg/redis"
)

type Config struct {
	TraefikProxyURL string
	RedisAddress    string
	RedisPort       string
	RedisDataBase   string
	CacheTTL        string
}

func CreateConfig() *Config {
	return &Config{}
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if config.RedisAddress == "" {
		slog.Error("[REDIS CONFIG] The Redis address has not been set")
		return nil, fmt.Errorf("[REDIS CONFIG] The Redis address has not been set")
	}

	config.CacheTTL = common.VerifyEmpty(config.CacheTTL, "60")
	config.TraefikProxyURL = common.VerifyEmpty(config.TraefikProxyURL, "https://traefik.traefik-controller.svc.cluster.local:443")
	config.RedisPort = common.VerifyEmpty(config.RedisPort, "6379")

	slog.Info("instanced plugin", "name", name)

	traefikTarget, err := url.Parse(config.TraefikProxyURL)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, err
	}

	redisConn := redis.NewConnection(config.RedisAddress, config.RedisPort, config.CacheTTL)

	targetProxy := &pkg.Proxy{
		ProxyURL:  traefikTarget,
		RedisConn: redisConn,
	}

	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Rewrite: targetProxy.RewriteProxy(),
	}

	bg := pkg.New(next, proxy, name)

	return bg, nil
}
