package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type ServerConfig struct {
	Name      string `mapstructure:"name"`
	Type      string `mapstructure:"type"`
	Address   string `mapstructure:"address"`
	Port      int    `mapstructure:"port"`
	UUID      string `mapstructure:"uuid,omitempty"`
	Password  string `mapstructure:"password,omitempty"`
	Method    string `mapstructure:"method,omitempty"`
	Security  string `mapstructure:"security,omitempty"`
	Network   string `mapstructure:"network,omitempty"`
	Host      string `mapstructure:"host,omitempty"`
	Path      string `mapstructure:"path,omitempty"`
	AlterID   int    `mapstructure:"alterId,omitempty"`
	TLS       bool   `mapstructure:"tls,omitempty"`
	Latency   string `mapstructure:"-"` // Latency is tested at runtime, not saved
}

type RuleConfig struct {
	Type   string   `mapstructure:"type"`
	Action string   `mapstructure:"action"`
	Values []string `mapstructure:"values"`
}

type SubscriptionConfig struct {
	URL  string `mapstructure:"url"`
	Name string `mapstructure:"name"`
}

type AppConfig struct {
	Mode          string               `mapstructure:"mode"`
	Servers       []ServerConfig       `mapstructure:"servers"`
	Rules         []RuleConfig         `mapstructure:"rules"`
	Subscriptions []SubscriptionConfig `mapstructure:"subscriptions"`
	ActiveIndex   int                  `mapstructure:"active_index"`
}

func LoadConfig(path string) (*AppConfig, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		// If the file doesn't exist, create a default one.
		return &AppConfig{Servers: []ServerConfig{}, Subscriptions: []SubscriptionConfig{}}, nil
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
	viper.Set("servers", cfg.Servers)
	viper.Set("rules", cfg.Rules)
	viper.Set("subscriptions", cfg.Subscriptions)
	viper.Set("active_index", cfg.ActiveIndex)
	return viper.WriteConfigAs(path) // Use WriteConfigAs to create the file if it doesn't exist
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
