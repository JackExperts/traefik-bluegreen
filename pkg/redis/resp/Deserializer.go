package redis

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/JackExperts/traefik-bluegreen/pkg/redis/models"
)

type DataType byte

const (
	Array DataType = '*'
	Error DataType = '-'
)

func getRedisRESP(rd *bufio.Reader) ([]byte, error) {
	resp, err := rd.ReadBytes('\n')

	if err != nil {
		slog.Error("[REDIS OPERATION] erro na resposta do redis", "error", err)
		return nil, err
	}

	return resp[:len(resp)-2], nil // *6

}

func readBulkString(rd *bufio.Reader) (string, error) {
	_, err := rd.ReadString('\n')

	if err != nil {
		slog.Error("[REDIS OPERATION] erro na leitura de valores vindos do Redis", "error", err)
		return "", err
	}

	value, err := rd.ReadString('\n')
	if err != nil {
		slog.Error("[REDIS OPERATION] erro na leitura de valores vindos do Redis", "error", err)
		return "", err
	}

	return strings.TrimSpace(value), nil

}

func readPair(rd *bufio.Reader) (string, string, error) {
	field, err := readBulkString(rd)
	if err != nil {
		return "", "", err
	}

	value, err := readBulkString(rd)
	if err != nil {
		return "", "", err
	}

	return field, value, nil
}

func deserializeArray(rd *bufio.Reader) (*models.TenantSlot, error) {
	returnBytes, _ := getRedisRESP(rd)
	returnBytesToInt, err := strconv.Atoi(string(returnBytes))

	if err != nil {
		slog.Error("erro na conversão de tipo", "error", err)
		return nil, err
	}

	if returnBytesToInt == 0 {
		slog.Error("[REDIS OPERATION] valor não encontrado no redis")
		return nil, fmt.Errorf("valor não encontrado no redis")
	}

	tenantMap := make(map[string]string)

	for i := 0; i < returnBytesToInt/2; i++ {
		field, value, err := readPair(rd)
		if err != nil {
			slog.Error("[REDIS OPERATION] erro na leitura de valores vindos do Redis", "error", err)
			return nil, err
		}

		tenantMap[field] = value
	}

	return &models.TenantSlot{
		TenantID: tenantMap["tenant"],
		AppName:  tenantMap["app"],
		Slot:     tenantMap["slot"],
	}, nil

}

func Deserializer(rd io.Reader) (*models.TenantSlot, error) {
	reader := bufio.NewReader(rd)

	respType, err := reader.ReadByte()

	if err != nil {
		return nil, err
	}

	switch DataType(respType) {
	case Array:
		return deserializeArray(reader)
	// TODO: Implementar retorno para error na resposta do Redis
	// TODO: Implementar retorno default
	default:
		return nil, nil
	}

}
