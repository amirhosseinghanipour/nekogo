package gui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/amirhosseinghanipour/nekogo/config"
	"github.com/amirhosseinghanipour/nekogo/core"
)

type BlackTheme struct{}

func (b *BlackTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNameBackground || n == theme.ColorNameButton {
		return color.Black
	}
	if n == theme.ColorNameDisabled || n == theme.ColorNameDisabledButton {
		return color.RGBA{30, 30, 30, 255}
	}
	if n == theme.ColorNameForeground || n == theme.ColorNameButtonText {
		return color.White
	}
	return theme.DefaultTheme().Color(n, v)
}
func (b *BlackTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (b *BlackTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (b *BlackTheme) Size(n fyne.ThemeSizeName) float32       { return theme.DefaultTheme().Size(n) }

func RunGUI() {
	a := app.New()
	w := a.NewWindow("NekoGo - Tunnel App")
	w.Resize(fyne.NewSize(600, 500))
	bg := canvas.NewRectangle(color.RGBA{0, 0, 0, 255})
	title := canvas.NewText("NekoGo", color.White)
	title.TextSize = 32
	status := widget.NewLabel("Idle")
	logBox := widget.NewMultiLineEntry()
	logBox.SetText("")
	logBox.Disable()
	statsLabel := widget.NewLabel("Stats: 0 bytes sent, 0 bytes received")

	// Load config
	cfg, err := config.LoadConfig("nekogo.yaml")
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load config: %w", err), w)
		cfg = &config.AppConfig{}
	}

	// Proxy type dropdown and address/port entry
	proxyTypes := []string{"socks5", "http", "https"}
	proxyTypeSelect := widget.NewSelect(proxyTypes, func(selected string) {})
	proxyAddrEntry := widget.NewEntry()
	proxyPortEntry := widget.NewEntry()
	proxyTypeSelect.SetSelected("socks5")
	proxyAddrEntry.SetPlaceHolder("Proxy address (e.g. 127.0.0.1)")
	proxyPortEntry.SetPlaceHolder("Port (e.g. 1080)")

	// If a server is selected, update fields
	updateProxyFields := func() {
		if cfg.ActiveIndex >= 0 && cfg.ActiveIndex < len(cfg.Servers) {
			srv := cfg.Servers[cfg.ActiveIndex]
			proxyTypeSelect.SetSelected(srv.Type)
			proxyAddrEntry.SetText(srv.Address)
			proxyPortEntry.SetText(fmt.Sprintf("%d", srv.Port))
		}
	}
	serverList := widget.NewList(
		func() int { return len(cfg.Servers) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i int, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("%d: %s (%s)", i, cfg.Servers[i].Name, cfg.Servers[i].Type))
		},
	)
	serverList.OnSelected = func(id int) {
		cfg.ActiveIndex = id
		status.SetText(fmt.Sprintf("Selected server: %s", cfg.Servers[id].Name))
		updateProxyFields()
	}

	// Save changes to selected server
	saveServerBtn := widget.NewButton("Save Server", func() {
		if cfg.ActiveIndex >= 0 && cfg.ActiveIndex < len(cfg.Servers) {
			srv := &cfg.Servers[cfg.ActiveIndex]
			srv.Type = proxyTypeSelect.Selected
			srv.Address = proxyAddrEntry.Text
			fmt.Sscanf(proxyPortEntry.Text, "%d", &srv.Port)
			if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
				dialog.ShowError(err, w)
				status.SetText("Failed to save config")
			} else {
				status.SetText("Server updated and saved to nekogo.yaml")
			}
		}
	})

	// Add new server (in memory only for now)
	addServerBtn := widget.NewButton("Add Server", func() {
		newSrv := config.ServerConfig{
			Name:    fmt.Sprintf("Server %d", len(cfg.Servers)+1),
			Type:    proxyTypeSelect.Selected,
			Address: proxyAddrEntry.Text,
			Port:    0,
		}
		fmt.Sscanf(proxyPortEntry.Text, "%d", &newSrv.Port)
		cfg.Servers = append(cfg.Servers, newSrv)
		serverList.Refresh()
		if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
			dialog.ShowError(err, w)
			status.SetText("Failed to save new server")
		} else {
			status.SetText("Server added and saved to nekogo.yaml")
		}
	})

	// Rule management (display only)
	ruleList := widget.NewList(
		func() int { return len(cfg.Rules) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i int, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("%s: %v -> %s", cfg.Rules[i].Type, cfg.Rules[i].Values, cfg.Rules[i].Action))
		},
	)

	tunBtn := widget.NewButton("TUN Mode", func() {
		go func() {
			status.SetText("Starting TUN mode...")
			err := core.StartTUNWithConfig(cfg)
			if err != nil {
				status.SetText("TUN error")
				dialog.ShowError(err, w)
				logBox.SetText(logBox.Text + "\n" + err.Error())
			} else {
				status.SetText("TUN running")
				logBox.SetText(logBox.Text + "\nTUN started")
			}
		}()
	})
	proxyBtn := widget.NewButton("Proxy Mode", func() {
		go func() {
			status.SetText("Starting Proxy mode...")
			proxyType := proxyTypeSelect.Selected
			proxyAddr := fmt.Sprintf("%s:%s", proxyAddrEntry.Text, proxyPortEntry.Text)
			err := core.StartProxy(proxyType, proxyAddr)
			if err != nil {
				status.SetText("Proxy error")
				dialog.ShowError(err, w)
				logBox.SetText(logBox.Text + "\n" + err.Error())
			} else {
				status.SetText(fmt.Sprintf("Proxy running on %s://%s", proxyType, proxyAddr))
				logBox.SetText(logBox.Text + fmt.Sprintf("\nProxy started on %s://%s", proxyType, proxyAddr))
			}
		}()
	})
	sysTunnelBtn := widget.NewButton("System Tunnel", func() {
		go func() {
			status.SetText("Starting System Tunnel...")
			err := core.StartSystemTunnel()
			if err != nil {
				status.SetText("System Tunnel error")
				dialog.ShowError(err, w)
				logBox.SetText(logBox.Text + "\n" + err.Error())
			} else {
				status.SetText("System Tunnel running")
				logBox.SetText(logBox.Text + "\nSystem Tunnel started")
			}
		}()
	})
	for _, btn := range []*widget.Button{tunBtn, proxyBtn, sysTunnelBtn} {
		btn.Importance = widget.HighImportance
		btn.Style = widget.PrimaryButton
	}

	// Real-time stats
	go func() {
		for {
			time.Sleep(time.Second)
			statsLabel.SetText(fmt.Sprintf("Stats: %d bytes sent, %d bytes received", core.Stats.BytesSent, core.Stats.BytesReceived))
		}
	}()

	content := container.NewVBox(
		container.NewCenter(title),
		widget.NewLabel("Select tunnel mode:"),
		container.NewHBox(tunBtn, proxyBtn, sysTunnelBtn),
		widget.NewLabel("Proxy Type:"),
		proxyTypeSelect,
		widget.NewLabel("Proxy Address:"),
		proxyAddrEntry,
		widget.NewLabel("Proxy Port:"),
		proxyPortEntry,
		container.NewHBox(saveServerBtn, addServerBtn),
		widget.NewLabel("Servers:"),
		serverList,
		widget.NewLabel("Rules:"),
		ruleList,
		widget.NewLabel("Status:"),
		status,
		widget.NewLabel("Logs:"),
		logBox,
		statsLabel,
	)
	w.SetContent(container.NewMax(bg, container.NewVBox(
		widget.NewLabel(" "),
		content,
	)))
	w.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("File",
			fyne.NewMenuItem("Quit", func() { a.Quit() }),
		),
	))
	w.SetTheme(&BlackTheme{})
	w.ShowAndRun()
}