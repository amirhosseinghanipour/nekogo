package cmd

import (
	"fmt"
	"os"
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
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start tunnel (CLI mode)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting tunnel in CLI mode (not implemented yet)")
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configs",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Config management (not implemented yet)")
	},
} 