// mylan shows the devices on the local network in a panel of the menu bar.
package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

// shell lives as long as the app: the tray icon, the open panel, and the
// state the panel shows.
type shell struct {
	a     *app
	tray  *mygo.Tray
	panel *mygo.Window
}

func (s *shell) ready() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	cfg := loadConfig(dir)
	a := &app{
		cfg:     cfg,
		dataDir: dir,
		clock:   time.Now,
		login:   mygo.App.OpenAtLogin(),
	}
	s.a = a
	a.post = func(fn func()) {
		mygo.RunOnMain(func() {
			fn()
			s.updateTray()
			if s.panel != nil {
				s.panel.Update(func() {})
			}
		})
	}
	a.spawn = func(fn func()) { go fn() }
	a.quit = mygo.App.Quit
	a.close = func() {
		if s.panel != nil {
			s.panel.Close()
		}
	}
	a.changed = func(cfg config) {
		if err := writeConfig(dir, cfg); err != nil {
			log.Print("config: ", err)
		}
		s.updateTray()
	}

	s.tray, err = mygo.NewTray(mygo.TrayOptions{
		Icon:    trayIcon(),
		ToolTip: "mylan",
	})
	if err != nil {
		log.Fatal("tray: ", err)
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开 mylan", Click: func(*mygo.MenuItem, *mygo.Window) { s.toggle() }},
		{Label: "刷新", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) { a.refresh() }},
		updater.MenuItem(),
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
	if runtime.GOOS == "linux" {
		s.tray.SetMenu(menu)
	} else {
		s.tray.OnClick(s.toggle)
		s.tray.OnRightClick(func() {
			if s.panel != nil {
				s.panel.Close()
			}
			s.tray.SetMenu(menu)
			defer s.tray.SetMenu(nil)
			s.tray.PopUpMenu()
		})
	}
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { s.tray.Destroy() })
	s.updateTray()

	go func() {
		tick := time.NewTicker(15 * time.Second)
		for range tick.C {
			mygo.RunOnMain(func() {
				every := time.Duration(a.cfg.AutoRefresh) * time.Second
				if every <= 0 || a.scanning || a.loadedAt.IsZero() {
					return
				}
				if a.now().Sub(a.loadedAt) >= every {
					a.refresh()
				}
			})
		}
	}()
}

// toggle opens the panel below the icon, or closes it.
func (s *shell) toggle() {
	if s.panel != nil {
		s.panel.Close()
		return
	}
	if s.a.stale() && !s.a.scanning {
		s.a.refresh()
	}
	s.panel = mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:      s.tray,
		Placement: mygo.PlacementBottom,
		Gap:       4,
		Width:     panelWidth,
		Height:    panelHeight,
		Focusable: true,
		Shadow:    true,
		Popover:   true,
		Content:   ui.View(s.a.view),
	})
	s.panel.OnClosed(func() { s.panel = nil })
}

func (s *shell) updateTray() {
	if s.tray == nil || s.a == nil {
		return
	}
	n := s.a.onlineCount()
	title := ""
	if runtime.GOOS != "windows" && !s.a.cfg.HideMenuCount && n > 0 {
		title = fmt.Sprint(n)
	}
	tip := "mylan"
	if sub := s.a.report.Scanned.Subnet; sub != "" {
		tip = fmt.Sprintf("%s · 在线 %d", sub, n)
	}
	s.tray.SetTitle(title)
	s.tray.SetToolTip(tip)
}

func main() {
	s := &shell{}
	mygo.Use(native.Plugin)
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(s.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
