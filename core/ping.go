package core

import (
	"fmt"
	"net"
	"time"

	"github.com/amirhosseinghanipour/nekogo/config"
)

// TestServerLatency measures the TCP handshake time to a server.
func TestServerLatency(server config.ServerConfig) (time.Duration, error) {
	address := fmt.Sprintf("%s:%d", server.Address, server.Port)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return 0, err
	}
	conn.Close()

	return time.Since(start), nil
}
