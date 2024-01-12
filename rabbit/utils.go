package infrarabbit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func createAMQPURL(cfg *ConnectionConfig) (string, error) {
	host, port := getHostPort(cfg.Address)
	if host == "" {
		return "", errors.New("invalid AMQP host")
	}
	if port == 0 {
		return "", errors.New("invalid AMQP port")
	}

	username := defaultUser
	if cfg.Username != "" {
		username = cfg.Username
	}

	password := defaultPassword
	if cfg.Password != "" {
		password = cfg.Password
	}

	vhost := defaultVHost
	if cfg.Vhost != "" {
		vhost = cfg.Vhost
	}

	return fmt.Sprintf(
		"amqp://%s:%s@%s:%d%s",
		username,
		password,
		host,
		port,
		vhost), nil
}

func getHostPort(address string) (string, int) {
	hostPort := strings.Split(address, ":")
	if len(hostPort) != 2 {
		return "", 0
	}

	host := hostPort[0]
	port, _ := strconv.ParseInt(hostPort[1], 10, 64)
	if port <= 0 {
		return "", 0
	}

	return host, int(port)
}
