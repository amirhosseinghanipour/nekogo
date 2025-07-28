package gui

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/amirhosseinghanipour/nekogo/config"
	"github.com/amirhosseinghanipour/nekogo/core"
)

type BlackTheme struct{}

func (b *BlackTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNameBackground {
		return color.NRGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 0xff}
	}
	if n == theme.ColorNameButton {
		return color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xff}
	}
	if n == theme.ColorNameDisabled || n == theme.ColorNameDisabledButton {
		return color.NRGBA{R: 0x28, G: 0x28, B: 0x28, A: 0xff}
	}
	if n == theme.ColorNameForeground {
		return color.White
	}
	return theme.DefaultTheme().Color(n, v)
}
func (b *BlackTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (b *BlackTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (b *BlackTheme) Size(n fyne.ThemeSizeName) float32       { return theme.DefaultTheme().Size(n) }

var (
	cfg      *config.AppConfig
	stopChan chan struct{}
	mu       sync.Mutex
)

func RunGUI() {
	a := app.New()
	a.Settings().SetTheme(&BlackTheme{})
	w := a.NewWindow("NekoGo")
	w.Resize(fyne.NewSize(900, 700))

	var err error
	cfg, err = config.LoadConfig("nekogo.yaml")
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to load config: %w", err), w)
	}

	statusLabel := widget.NewLabel("Status: Idle")
	statsLabel := widget.NewLabel("Sent: 0.00 KB/s | Received: 0.00 KB/s")
	serverList := widget.NewList(
		func() int { return len(cfg.Servers) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewLabel("Server Name"), widget.NewLabel("Latency"))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			c := o.(*fyne.Container)
			c.Objects[0].(*widget.Label).SetText(cfg.Servers[i].Name)
			c.Objects[1].(*widget.Label).SetText(cfg.Servers[i].Latency)
		},
	)

	startStopBtn := widget.NewButton("Start", nil)
	startStopBtn.Importance = widget.HighImportance

	modeSelector := widget.NewRadioGroup([]string{"tun", "proxy"}, func(selected string) {
		cfg.Mode = selected
		if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), w)
		}
	})
	modeSelector.SetSelected(cfg.Mode)
	modeSelector.Horizontal = true

	rulesLabel := widget.NewLabel(buildRulesString(cfg.Rules))
	rulesLabel.Wrapping = fyne.TextWrapWord

	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.MediaPlayIcon(), func() { // Test Latency
			for i := range cfg.Servers {
				go func(index int) {
					latency, err := core.TestServerLatency(cfg.Servers[index])
					mu.Lock()
					if err != nil {
						cfg.Servers[index].Latency = "Timeout"
					} else {
						cfg.Servers[index].Latency = fmt.Sprintf("%d ms", latency.Milliseconds())
					}
					mu.Unlock()
					serverList.Refresh()
				}(i)
			}
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.SettingsIcon(), func() { // Set System Proxy
			// Placeholder for system proxy logic.
			// This will require new functions in the `core` package.
			dialog.ShowInformation("Not Implemented", "System proxy management is not yet implemented.", w)
		}),
	)

	topBox := container.NewVBox(
		toolbar,
		container.NewGridWithColumns(2, startStopBtn, statusLabel),
		modeSelector,
	)

	content := container.NewBorder(
		topBox,
		container.NewVBox(statsLabel, widget.NewSeparator(), rulesLabel), nil, nil,
		serverList,
	)

	startStopBtn.OnTapped = func() {
		if stopChan != nil { // Currently running, so stop it
			close(stopChan)
			stopChan = nil
			startStopBtn.SetText("Start")
			statusLabel.SetText("Status: Stopped")
		} else { // Not running, so start it
			if len(cfg.Servers) == 0 || cfg.ActiveIndex < 0 {
				dialog.ShowInformation("Error", "No server selected", w)
				return
			}
			stopChan = make(chan struct{})
			go func() {
				statusLabel.SetText("Status: Running...")
				var err error
				if cfg.Mode == "tun" {
					err = core.StartTUNWithConfig(cfg, stopChan)
				} else if cfg.Mode == "proxy" {
					activeServer := cfg.Servers[cfg.ActiveIndex]
					proxyAddr := fmt.Sprintf("%s:%d", activeServer.Address, activeServer.Port)
					err = core.StartProxy(activeServer.Type, proxyAddr)
				} else {
					err = fmt.Errorf("unsupported mode: %s", cfg.Mode)
				}

				if err != nil {
					statusLabel.SetText(fmt.Sprintf("Status: Error - %v", err))
				} else {
					statusLabel.SetText("Status: Idle")
				}
			}()
			startStopBtn.SetText("Stop")
		}
	}

	serverList.OnSelected = func(id widget.ListItemID) {
		cfg.ActiveIndex = id
		config.SaveConfig("nekogo.yaml", cfg)
	}

	w.Canvas().AddShortcut(&fyne.ShortcutPaste{}, func(shortcut fyne.Shortcut) {
		importFromClipboard(w.Clipboard().Content(), w, serverList)
	})

	w.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("File",
			fyne.NewMenuItem("Restart App", func() {
				a.Quit()
				cmd := exec.Command(os.Args[0], os.Args[1:]...)
				cmd.Start()
			}),
			fyne.NewMenuItem("Quit", func() { a.Quit() }),
		),
		fyne.NewMenu("Edit",
			fyne.NewMenuItem("Import from Clipboard", func() {
				importFromClipboard(w.Clipboard().Content(), w, serverList)
			}),
			fyne.NewMenuItem("Remove Active Server", func() {
				if cfg.ActiveIndex >= 0 && cfg.ActiveIndex < len(cfg.Servers) {
					cfg.Servers = append(cfg.Servers[:cfg.ActiveIndex], cfg.Servers[cfg.ActiveIndex+1:]...)
					if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
						dialog.ShowError(fmt.Errorf("failed to save config: %w", err), w)
						return
					}
					serverList.Refresh()
				}
			}),
			fyne.NewMenuItem("Remove Duplicates", func() {
				removeDuplicates()
				if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
					dialog.ShowError(fmt.Errorf("failed to save config: %w", err), w)
					return
				}
				serverList.Refresh()
				dialog.ShowInformation("Success", "Duplicate servers removed.", w)
			}),
			fyne.NewMenuItem("Clear All Servers", func() {
				dialog.ShowConfirm("Confirm", "Are you sure you want to remove all servers?", func(confirm bool) {
					if !confirm {
						return
					}
					cfg.Servers = []config.ServerConfig{}
					cfg.ActiveIndex = 0
					if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
						dialog.ShowError(fmt.Errorf("failed to save config: %w", err), w)
						return
					}
					serverList.Refresh()
				}, w)
			}),
		),
	))

	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			statsLabel.SetText(fmt.Sprintf("Sent: %s | Received: %s", core.Stats.SentRate, core.Stats.ReceivedRate))
		}
	}()

	w.SetContent(content)
	w.ShowAndRun()
}

func importFromClipboard(content string, w fyne.Window, serverList *widget.List) {
	if content == "" {
		return
	}
	newServers, err := core.ParseServers(content)
	if err != nil {
		dialog.ShowError(fmt.Errorf("import failed: %w", err), w)
		return
	}
	if len(newServers) > 0 {
		cfg.Servers = append(cfg.Servers, newServers...)
		if err := config.SaveConfig("nekogo.yaml", cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), w)
			return
		}
		serverList.Refresh()
		dialog.ShowInformation("Success", fmt.Sprintf("Added %d new servers.", len(newServers)), w)
	}
}

func removeDuplicates() {
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
}

func buildRulesString(rules []config.RuleConfig) string {
	if len(rules) == 0 {
		return "No rules configured."
	}
	var b strings.Builder
	b.WriteString("Routing Rules:\n")
	for _, rule := range rules {
		b.WriteString(fmt.Sprintf("  - Type: %s, Action: %s, Values: %v\n", rule.Type, rule.Action, rule.Values))
	}
	return b.String()
}
