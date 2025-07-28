package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type ServerConfig struct {
	Name    string `mapstructure:"name"`
	Type    string `mapstructure:"type"` // socks5, http, https, shadowsocks
	Address string `mapstructure:"address"`
	Port    int    `mapstructure:"port"`
	Method  string `mapstructure:"method,omitempty"`
	Password string `mapstructure:"password,omitempty"`
}

type RuleConfig struct {
	Type   string        `mapstructure:"type"`
	Values []string      `mapstructure:"values"`
	Action string        `mapstructure:"action"`
}

type AppConfig struct {
	Mode        string         `mapstructure:"mode"`
	ProxyAddr   string         `mapstructure:"proxy_addr"`
	Servers     []ServerConfig `mapstructure:"servers"`
	ActiveIndex int            `mapstructure:"active_index"`
	Rules       []RuleConfig   `mapstructure:"rules"`
}

func LoadConfig(path string) (*AppConfig, error) {
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var cfg AppConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *AppConfig) error {
	viper.SetConfigFile(path)
	viper.Set("mode", cfg.Mode)
	viper.Set("proxy_addr", cfg.ProxyAddr)
	viper.Set("servers", cfg.Servers)
	viper.Set("active_index", cfg.ActiveIndex)
	viper.Set("rules", cfg.Rules)
	return viper.WriteConfigAs(path)
}

func (cfg *AppConfig) Validate() error {
	if len(cfg.Servers) == 0 {
		return fmt.Errorf("no servers configured")
	}
	if cfg.ActiveIndex < 0 || cfg.ActiveIndex >= len(cfg.Servers) {
		return fmt.Errorf("invalid active server index")
	}
	return nil
} 