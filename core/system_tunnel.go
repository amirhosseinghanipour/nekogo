package core

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
)

func StartSystemTunnel() error {
	osType := runtime.GOOS
	switch osType {
	case "linux":
		log.Println("Setting system default route to TUN (Linux MVP)")
		cmd := exec.Command("sudo", "ip", "route", "replace", "default", "dev", "nekogo-tun")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set default route: %w", err)
		}
		return nil
	case "darwin":
		return fmt.Errorf("System tunnel not implemented for macOS yet")
	case "windows":
		return fmt.Errorf("System tunnel not implemented for Windows yet")
	default:
		return fmt.Errorf("Unsupported OS: %s", osType)
	}
}
