package commands

import (
	"log/slog"
	"net"

	"github.com/PHBueno/traefik-bluegreen/pkg/redis/models"
	resp "github.com/PHBueno/traefik-bluegreen/pkg/redis/resp"
)

func HGetAll(conn net.Conn, key string) (*models.TenantSlot, error) {
	_, err := conn.Write([]byte(resp.Serializer("HGETALL", key)))

	if err != nil {
		slog.Error("[REDIS OPERATION] => erro para buscar valor no redis", "error", err)
		return nil, err
	}

	tenantModel, err := resp.Deserializer(conn)

	if err != nil {
		slog.Error("[REDIS OPERATION] => erro na resposta do redis", "err", err)
		return nil, err
	}

	return tenantModel, nil

}
