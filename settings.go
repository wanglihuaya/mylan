package main

import (
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

func (a *app) settings(c *ui.Context, k ink) {
	ui.Row(c).Padding(12, 12, 4, 8).Children(func() {
		textButton(c.Key("back"), k, "设备", iconBack, true).OnClick(a.back)
	})
	ui.Scroll(c.Key("settings")).Grow(1).Padding(8, gutter, gutter, gutter).Gap(22).Children(func() {
		ui.Text(c, "设置").FontSize(17).FontWeight(650)

		section(c, k, "菜单栏", func() {
			show := !a.cfg.HideMenuCount
			switchRow(c.Key("menu-count"), k, "显示数量", "在图标后面显示在线设备数。", &show, func() {
				a.cfg.HideMenuCount = !show
				a.persist()
			})
		})

		section(c, k, "扫描", func() {
			mode := 0
			if a.cfg.Fast {
				mode = 1
			}
			ui.Segmented(c.Key("speed"), &mode, "完整", "快速").Label("扫描方式").AlignSelf(ui.Start).
				OnChange(func() {
					a.cfg.Fast = mode == 1
					a.persist()
				})
			ui.Text(c, "快速模式只读 ARP 缓存和 Bonjour，大约 3 秒，不探测端口。下次刷新生效。").
				FontSize(11).TextColor(k.muted).LineHeight(1.4)

			stale := a.cfg.Stale
			switchRow(c.Key("stale"), k, "包含休眠设备", "列出 ARP 缓存里 ping 不通的设备。", &stale, func() {
				a.cfg.Stale = stale
				a.persist()
			})
			names := a.cfg.Names
			switchRow(c.Key("names"), k, "用 SMB 补名称", "为没有名字的 Windows 电脑读取机器名。", &names, func() {
				a.cfg.Names = names
				a.persist()
			})
			field(c, k, "网段", "留空则跟随当前默认路由。例如 192.168.1.0/24。", func() {
				ui.TextInput(c.Key("subnet"), &a.editSubnet).Label("网段").Placeholder("自动").
					OnSubmit(func() {
						a.cfg.Subnet = a.editSubnet
						a.persist()
					})
			})
		})

		section(c, k, "自动刷新", func() {
			idx := 0
			switch a.cfg.AutoRefresh {
			case 60:
				idx = 1
			case 300:
				idx = 2
			}
			ui.Segmented(c.Key("auto"), &idx, "关", "1 分钟", "5 分钟").Label("自动刷新").AlignSelf(ui.Start).
				OnChange(func() {
					a.cfg.AutoRefresh = []int{0, 60, 300}[idx]
					a.persist()
				})
			ui.Text(c, "完整扫描会 ping 整个网段。默认只在打开面板、且结果超过 2 分钟时自动扫一次。").
				FontSize(11).TextColor(k.muted).LineHeight(1.4)
		})
	})
	ui.Row(c).Padding(10, gutter, 12, gutter).Gap(8).AlignItems(ui.Center).FontSize(11).TextColor(k.faint).
		BorderWidth(1, 0, 0, 0).BorderColor(k.line).Children(func() {
		ui.Text(c, "mylan "+version).Grow(1)
		textButton(c.Key("updates"), k, "检查更新", nil, false).OnClick(func() {
			updater.CheckForUpdates()
		})
		textButton(c.Key("quit"), k, "退出", nil, false).OnClick(func() {
			if a.quit != nil {
				a.quit()
			}
		})
	})
}

func section(c *ui.Context, k ink, title string, build func()) {
	ui.Column(c).Gap(10).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(11).FontWeight(500).TextColor(k.faint)
		}
		build()
	})
}

func field(c *ui.Context, k ink, label, hint string, input func()) {
	ui.Column(c).Gap(5).Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(500)
		input()
		if hint != "" {
			ui.Text(c, hint).FontSize(11).TextColor(k.muted).LineHeight(1.35)
		}
	})
}

func switchRow(c *ui.Context, k ink, label, detail string, on *bool, changed func()) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(1).OnClick(func() {
			*on = !*on
			changed()
		}).Children(func() {
			ui.Text(c, label).SingleLine()
			if detail != "" {
				ui.Text(c, detail).FontSize(11).TextColor(k.muted).SingleLine()
			}
		})
		ui.Switch(c, on).Label(label).OnChange(changed)
	})
}
