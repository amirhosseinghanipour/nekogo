package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/amirhosseinghanipour/nekogo/config"
	"github.com/amirhosseinghanipour/nekogo/core"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "nekogo",
	Short: "NekoGo CLI",
	Long:  `NekoGo - Modern Tunnel App (CLI)`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(getCmd)
	configCmd.AddCommand(setActiveCmd)
	configCmd.AddCommand(clearCmd)
	configCmd.AddCommand(dedupeCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start tunnel (CLI mode)",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig("nekogo.yaml")
		if err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			os.Exit(1)
		}

		if err := cfg.Validate(); err != nil {
			fmt.Printf("Invalid config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Starting NekoGo in %s mode...\n", cfg.Mode)
		if cfg.Mode == "tun" {
			if err := core.StartTUNWithConfig(cfg, nil); err != nil {
				fmt.Printf("Error starting TUN mode: %v\n", err)
				os.Exit(1)
			}
		} else if cfg.Mode == "proxy" {
			activeServer := cfg.Servers[cfg.ActiveIndex]
			proxyAddr := fmt.Sprintf("%s:%d", activeServer.Address, activeServer.Port)
			if err := core.StartProxy(activeServer.Type, proxyAddr); err != nil {
				fmt.Printf("Error starting proxy mode: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("Unsupported mode: %s\n", cfg.Mode)
			os.Exit(1)
		}
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configs",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get the current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig("nekogo.yaml")
		if err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Current Mode: %s\n", cfg.Mode)
		fmt.Printf("Active Server Index: %d\n", cfg.ActiveIndex)
		fmt.Println("Servers:")
		for i, srv := range cfg.Servers {
			fmt.Printf("  [%d] %s (%s) - %s:%d\n", i, srv.Name, srv.Type, srv.Address, srv.Port)
		}
		fmt.Println("Rules:")
		for _, rule := range cfg.Rules {
			fmt.Printf("  - Type: %s, Action: %s, Values: %v\n", rule.Type, rule.Action, rule.Values)
		}
	},
}

var setActiveCmd = &cobra.Command{
	Use:   "set-active [index]",
	Short: "Set the active server by index",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig("nekogo.yaml")
		if err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			os.Exit(1)
		}

		newIndex, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("Invalid index: %v\n", err)
			os.Exit(1)
		}

		if newIndex < 0 || newIndex >= len(cfg.Servers) {
			fmt.Println("Index out of range.")
			os.Exit(1)
		}

		cfg.ActiveIndex = newIndex
		if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
			fmt.Printf("Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Active server set to index %d.\n", newIndex)
	},
}

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove all servers from the configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig("nekogo.yaml")
		if err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			os.Exit(1)
		}

		cfg.Servers = []config.ServerConfig{}
		cfg.ActiveIndex = 0

		if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
			fmt.Printf("Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("All servers have been removed.")
	},
}

var dedupeCmd = &cobra.Command{
	Use:   "dedupe",
	Short: "Remove duplicate servers from the configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig("nekogo.yaml")
		if err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			os.Exit(1)
		}

		seen := make(map[string]bool)
		var result []config.ServerConfig
		for _, server := range cfg.Servers {
			identifier := fmt.Sprintf("%s:%d", server.Address, server.Port)
			if _, ok := seen[identifier]; !ok {
				seen[identifier] = true
				result = append(result, server)
			}
		}
		cfg.Servers = result

		if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
			fmt.Printf("Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Duplicate servers have been removed.")
	},
}
