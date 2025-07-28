package core

import (
	"log"
	"github.com/getlantern/systray"
	"github.com/amirhosseinghanipour/nekogo/assets"
)

var running bool

func RunTray(startTunnel func(), stopTunnel func()) {
	systray.Run(func() {
		systray.SetIcon(assets.IconData)
		systray.SetTitle("NekoGo")
		systray.SetTooltip("NekoGo Tunnel App")
		mStart := systray.AddMenuItem("Start Tunnel", "Start tunnel")
		mStop := systray.AddMenuItem("Stop Tunnel", "Stop tunnel")
		mStop.Disable()
		mQuit := systray.AddMenuItem("Quit", "Quit the app")
		go func() {
			for {
				select {
				case <-mStart.ClickedCh:
					if !running {
						log.Println("Tray: Starting tunnel")
						startTunnel()
						running = true
						mStart.Disable()
						mStop.Enable()
					}
				case <-mStop.ClickedCh:
					if running {
						log.Println("Tray: Stopping tunnel")
						stopTunnel()
						running = false
						mStart.Enable()
						mStop.Disable()
					}
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, nil)
}
