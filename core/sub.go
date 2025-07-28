package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/amirhosseinghanipour/nekogo/config"
)

func ParseSubscription(subURL string) ([]config.ServerConfig, error) {
	resp, err := http.Get(subURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscription body: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(body))
	var content string
	if err != nil {
		content = string(body)
	} else {
		content = string(decoded)
	}

	return ParseServers(content)
}

func ParseServers(serverData string) ([]config.ServerConfig, error) {
	var servers []config.ServerConfig
	lines := strings.Split(serverData, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		u, err := url.Parse(line)
		if err != nil {
			fmt.Printf("Skipping invalid server URI: %s\n", line)
			continue
		}

		var server config.ServerConfig
		server.Name = u.Fragment
		server.Type = u.Scheme

		switch u.Scheme {
		case "vless":
			parseVless(u, &server)
		case "vmess":
			parseVmess(line, &server)
		case "trojan":
			parseTrojan(u, &server)
		case "ss":
			parseShadowsocks(u, &server)
		default:
			fmt.Printf("Unsupported server type: %s\n", u.Scheme)
			continue
		}

		if server.Address != "" {
			if server.Name == "" {
				server.Name = fmt.Sprintf("%s-%s:%d", server.Type, server.Address, server.Port)
			}
			servers = append(servers, server)
		}
	}
	return servers, nil
}

func parseVless(u *url.URL, server *config.ServerConfig) {
	server.UUID = u.User.Username()
	server.Address = u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	server.Port = port
	q := u.Query()
	server.Network = q.Get("type")
	server.Security = q.Get("security")
	server.Path = q.Get("path")
	server.Host = q.Get("host")
	if server.Host == "" {
		server.Host = q.Get("sni")
	}
}

func parseVmess(line string, server *config.ServerConfig) {
	encodedPart := strings.TrimPrefix(line, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(encodedPart)
	if err != nil {
		fmt.Printf("Failed to decode VMess config: %v\n", err)
		return
	}
	var vmessConfig struct {
		Add  string      `json:"add"`
		Port json.Number `json:"port"`
		ID   string      `json:"id"`
		Aid  json.Number `json:"aid"`
		Net  string      `json:"net"`
		Host string      `json:"host"`
		Path string      `json:"path"`
		TLS  string      `json:"tls"`
		Ps   string      `json:"ps"`
	}
	if err := json.Unmarshal(decoded, &vmessConfig); err != nil {
		fmt.Printf("Failed to unmarshal VMess JSON: %v\n", err)
		return
	}
	server.Name = vmessConfig.Ps
	server.Address = vmessConfig.Add
	port, _ := vmessConfig.Port.Int64()
	server.Port = int(port)
	server.UUID = vmessConfig.ID
	aid, _ := vmessConfig.Aid.Int64()
	server.AlterID = int(aid)
	server.Network = vmessConfig.Net
	server.Security = vmessConfig.TLS
	server.Host = vmessConfig.Host
	server.Path = vmessConfig.Path
}

func parseTrojan(u *url.URL, server *config.ServerConfig) {
	if u.User != nil {
		server.Password, _ = u.User.Password()
	}
	server.Address = u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	server.Port = port
	q := u.Query()
	if sni := q.Get("sni"); sni != "" {
		server.Host = sni
	}
}

func parseShadowsocks(u *url.URL, server *config.ServerConfig) {
	server.Type = "shadowsocks"
	if u.User != nil {
		encodedPart := u.User.Username()
		decoded, err := base64.StdEncoding.DecodeString(encodedPart)
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				server.Method = parts[0]
				server.Password = parts[1]
			}
		} else {
			server.Method = u.User.Username()
			if p, ok := u.User.Password(); ok {
				server.Password = p
			}
		}
	}
	server.Address = u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	server.Port = port
}
