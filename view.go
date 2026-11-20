package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// ink is the panel's colors. The panel draws over the popover's material
// on macOS, so its tints are translucent.
type ink struct {
	text, muted, faint ui.Color
	hover, press, line ui.Color
	up, accent         ui.Color
	knob               ui.Color
}

func inkOf(t *ui.Theme) ink {
	k := ink{
		text:   t.Text,
		muted:  t.TextMuted,
		faint:  t.Text.Alpha(0.32),
		hover:  t.Text.Alpha(0.055),
		press:  t.Text.Alpha(0.09),
		line:   t.Text.Alpha(0.08),
		up:     ui.Hex("#1f9d55"),
		accent: t.Accent,
		knob:   t.Background,
	}
	if t.Dark {
		k.up = ui.Hex("#4ccf7f")
		k.knob = t.Text.Alpha(0.16)
	}
	return k
}

const (
	panelWidth  = 420
	panelHeight = 640
	gutter      = 16
)

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	k := inkOf(t)
	c.Root().Background(ui.Transparent)

	c.OnShortcut(ui.Cmd, ui.KeyR, a.refresh)
	if a.page == pageDevices {
		c.OnShortcut(0, ui.KeyEscape, func() {
			if a.close != nil {
				a.close()
			}
		})
	} else {
		c.OnShortcut(0, ui.KeyEscape, a.back)
	}
	c.OnShortcut(ui.Cmd, ui.KeyComma, a.openSettings)

	box := ui.Column(c).Fill().FontSize(13).TextColor(k.text)
	switch {
	case c.Vibrancy():
	case runtime.GOOS == "windows":
		box.Background(t.Background)
	default:
		box.Background(t.Background).Radius(12).Border(1, t.Border)
	}
	box.Children(func() {
		switch {
		case a.page == pageSettings:
			a.settings(c, k)
		case a.page == pageDetail:
			a.detail(c, k)
		default:
			a.header(c, k)
			a.list(c, k)
		}
	})
}

func (a *app) header(c *ui.Context, k ink) {
	ui.Row(c).Padding(12, 12, 8, gutter).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.TextInput(c.Key("search"), &a.query).Grow(1).Placeholder("搜索名称、IP、MAC").Label("搜索")
		a.sortMenu(c.Key("sort"), k)
		iconButton(c.Key("refresh"), k, iconRefresh, "刷新", a.scanning).OnClick(a.refresh)
		iconButton(c.Key("settings"), k, iconSettings, "设置", false).OnClick(a.openSettings)
	})
}

func (a *app) sortMenu(c *ui.Context, k ink) {
	label := "IP"
	switch a.cfg.Sort {
	case "name":
		label = "名称"
	case "type":
		label = "类型"
	}
	b := ui.ButtonBase(c).Padding(3, 8).Gap(4).Radius(6).AlignItems(ui.Center).
		FontSize(12).TextColor(k.muted).Label("排序").Tooltip("排序")
	if b.Hovered() || b.Pressed() {
		b.Background(k.hover).TextColor(k.text)
	}
	b.Children(func() {
		ui.Text(c, "排序 · "+label).NoWrap()
		ui.Icon(c, iconChevron).FontSize(10)
	})
	b.Menu(func(m *ui.Menu) {
		if m.Item("IP").Checked(a.cfg.Sort == "" || a.cfg.Sort == "ip").Chosen() {
			a.cfg.Sort = "ip"
			a.persist()
		}
		if m.Item("名称").Checked(a.cfg.Sort == "name").Chosen() {
			a.cfg.Sort = "name"
			a.persist()
		}
		if m.Item("类型").Checked(a.cfg.Sort == "type").Chosen() {
			a.cfg.Sort = "type"
			a.persist()
		}
	})
}

func iconButton(c *ui.Context, k ink, svg *ui.SVG, label string, busy bool) ui.Element {
	b := ui.ButtonBase(c).Size(28, 28).Center().Radius(7).Label(label).Tooltip(label)
	switch {
	case b.Pressed():
		b.Background(k.press)
	case b.Hovered():
		b.Background(k.hover)
	}
	b.Children(func() {
		ic := ui.Icon(c, svg).FontSize(15).TextColor(k.muted)
		if b.Hovered() {
			ic.TextColor(k.text)
		}
		if busy {
			ic.Rotate(ic.Loop("spin", 900*time.Millisecond, ui.Linear) * 360)
		}
	})
	return b
}

func textButton(c *ui.Context, k ink, label string, svg *ui.SVG, iconFirst bool) ui.Element {
	b := ui.ButtonBase(c).Padding(4, 8).Gap(3).Radius(6).FontSize(12).FontWeight(500).AlignItems(ui.Center)
	b.TextColor(k.muted)
	switch {
	case b.Pressed():
		b.Background(k.press).TextColor(k.text)
	case b.Hovered():
		b.Background(k.hover).TextColor(k.text)
	}
	b.Children(func() {
		if svg != nil && iconFirst {
			ui.Icon(c, svg).FontSize(14)
		}
		ui.Text(c, label).NoWrap()
		if svg != nil && !iconFirst {
			ui.Icon(c, svg).FontSize(12)
		}
	})
	return b
}

func (a *app) list(c *ui.Context, k ink) {
	switch {
	case len(a.report.Devices) == 0 && a.scanning:
		a.scanningBody(c, k)
		return
	case len(a.report.Devices) == 0 && a.scanErr != nil:
		a.problem(c, k, "扫描失败", a.scanErr.Error(), true)
		return
	case len(a.report.Devices) == 0:
		a.problem(c, k, "没有发现设备", "确认当前网段，或在设置里手动指定。", true)
		return
	}
	a.summary(c, k)
	vis := a.visible()
	if len(vis) == 0 {
		ui.Column(c).Grow(1).Center().Padding(24, 32).Children(func() {
			ui.Text(c, "没有匹配的设备").FontSize(14).FontWeight(600)
			ui.Text(c, "换个关键词，或点「全部」取消类型过滤。").FontSize(12).TextColor(k.muted).Margin(6, 0, 0, 0)
		})
	} else {
		list := ui.Scroll(c.Key("devices")).Grow(1).Padding(0, 6, 6, 6).Label("设备")
		list.Children(func() {
			for i, d := range vis {
				a.deviceRow(c.Key(deviceKey(d)+":"+fmt.Sprint(i)), k, d)
			}
		})
	}
	a.footer(c, k)
}

func (a *app) scanningBody(c *ui.Context, k ink) {
	ui.Column(c).Padding(4, gutter, 0, gutter).Children(func() {
		ui.Text(c, "正在扫描").FontSize(13).FontWeight(600)
		ui.Text(c, a.scanHint()).FontSize(12).TextColor(k.muted).Margin(2, 0, 8, 0)
	})
	ui.Column(c).Padding(0, 6).Children(func() {
		for i := range 6 {
			skeletonRow(c.Key(i), k)
		}
	})
}

func (a *app) scanHint() string {
	if a.cfg.Fast {
		return "快速模式，读取 ARP 与 Bonjour。"
	}
	return "正在 ping 网段并浏览 Bonjour，大约需要几秒。"
}

func (a *app) summary(c *ui.Context, k ink) {
	total := len(a.report.Devices)
	online := a.onlineCount()
	shown := len(a.visible())
	ui.Column(c).Padding(2, gutter, 8, gutter).Gap(8).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, fmt.Sprintf("发现 %d · 在线 %d", total, online)).FontWeight(600).FontFeatures("tnum")
			ui.Spacer(c)
			if a.report.Scanned.Duration > 0 {
				ui.Text(c, fmt.Sprintf("%.1fs", a.report.Scanned.Duration)).FontSize(11).TextColor(k.faint).FontFeatures("tnum")
			}
		})
		if a.query != "" || a.typeFilter != "" {
			ui.Text(c, fmt.Sprintf("显示 %d", shown)).FontSize(11).TextColor(k.muted).FontFeatures("tnum")
		}
		a.typeChips(c, k)
		if a.scanErr != nil {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconAlert).FontSize(13).TextColor(ui.Hex("#d64545"))
				ui.Text(c, a.scanErr.Error()).FontSize(12).TextColor(k.muted).Grow(1).Shrink(1).SingleLine()
			})
		} else if len(a.report.Warnings) > 0 {
			ui.Text(c, a.report.Warnings[0]).FontSize(11).TextColor(k.muted).SingleLine()
		}
	})
}

func (a *app) typeChips(c *ui.Context, k ink) {
	counts := map[string]int{}
	var order []string
	for _, d := range a.searched() {
		t := d.Type
		if t == "" {
			t = "unknown"
		}
		if counts[t] == 0 {
			order = append(order, t)
		}
		counts[t]++
	}
	// Stable chip order follows the classifier, not discovery order.
	sorted := make([]string, 0, len(order))
	for _, t := range append(append([]string{}, typeOrder...), order...) {
		if counts[t] == 0 {
			continue
		}
		seen := false
		for _, s := range sorted {
			if s == t {
				seen = true
				break
			}
		}
		if !seen {
			sorted = append(sorted, t)
		}
	}
	chips := append([]string{""}, sorted...)
	const per = 3
	for i := 0; i < len(chips); i += per {
		end := i + per
		if end > len(chips) {
			end = len(chips)
		}
		row := chips[i:end]
		ui.Row(c).Gap(6).Children(func() {
			for _, t := range row {
				a.typeChip(c.Key("chip:"+t), k, t, counts[t])
			}
		})
	}
}

func (a *app) typeChip(c *ui.Context, k ink, t string, n int) {
	label := "全部"
	count := len(a.searched())
	selected := a.typeFilter == ""
	if t != "" {
		label = typeLabels[t]
		if label == "" {
			label = t
		}
		count = n
		selected = a.typeFilter == t
	}
	b := ui.ButtonBase(c).Padding(3, 8).Radius(6).FontSize(11).Label(fmt.Sprintf("%s %d", label, count))
	if selected {
		b.Background(k.knob).TextColor(k.text).FontWeight(600)
	} else {
		b.TextColor(k.muted)
		if b.Hovered() || b.Pressed() {
			b.Background(k.hover).TextColor(k.text)
		}
	}
	b.OnClick(func() {
		if t == "" {
			a.typeFilter = ""
			return
		}
		a.toggleType(t)
	})
	b.Children(func() {
		ui.Text(c, fmt.Sprintf("%s %d", label, count)).NoWrap().FontFeatures("tnum")
	})
}

func (a *app) deviceRow(c *ui.Context, k ink, d device) {
	row := ui.ButtonBase(c).Padding(8, 10).Gap(10).Radius(8).AlignItems(ui.Center).Label(d.displayName())
	switch {
	case row.Pressed():
		row.Background(k.press)
	case row.Hovered():
		row.Background(k.hover)
	}
	row.OnClick(func() {
		a.selected = deviceKey(d)
		a.editNote = a.note(d)
		a.page = pageDetail
	})
	sub := d.typeLabel()
	if d.Vendor != "" {
		sub += " · " + d.Vendor
	}
	if note := a.note(d); note != "" {
		sub += " · " + note
	}
	row.Children(func() {
		dot := ui.Box(c).Size(7, 7).Radius(4)
		switch {
		case d.IsLocal:
			dot.Background(k.accent)
		case d.Online:
			dot.Background(k.up)
		default:
			dot.Background(k.faint)
		}
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, d.displayName()).FontWeight(500).SingleLine().Shrink(1)
				if d.IsLocal {
					ui.Text(c, "本机").FontSize(10).TextColor(k.muted).NoWrap()
				}
				if d.IsGateway {
					ui.Text(c, "网关").FontSize(10).TextColor(k.muted).NoWrap()
				}
				if a.watching(d.ip()) {
					ui.Text(c, "监控").FontSize(10).TextColor(k.accent).NoWrap()
				}
			})
			ui.Text(c, sub).FontSize(11).TextColor(k.muted).SingleLine()
		})
		ip := d.ip()
		if ip == "" {
			ip = "—"
		}
		ui.Text(c, ip).FontFeatures("tnum").FontWeight(500).NoWrap()
	})
}

func skeletonRow(c *ui.Context, k ink) {
	ui.Row(c).Padding(8, 10).Gap(10).Children(func() {
		ui.Box(c).Size(7, 7).Radius(4).Background(k.hover).Margin(6, 0, 0, 0)
		ui.Column(c).Grow(1).Gap(6).Children(func() {
			skeleton(c, k, 140, 11)
			skeleton(c, k, 80, 9)
		})
		skeleton(c, k, 96, 12)
	})
}

func skeleton(c *ui.Context, k ink, w, h float32) ui.Element {
	return ui.Box(c).Size(w, h).Radius(4).Background(k.hover)
}

func (a *app) footer(c *ui.Context, k ink) {
	if !a.loadedAt.IsZero() {
		c.After(10 * time.Second)
	}
	ui.Row(c).Padding(8, gutter, 10, gutter).Gap(8).FontSize(11).TextColor(k.faint).
		BorderWidth(1, 0, 0, 0).BorderColor(k.line).AlignItems(ui.Center).Children(func() {
		scan := a.report.Scanned
		label := "局域网"
		if scan.Subnet != "" {
			label = scan.Subnet
			if scan.Iface != "" {
				label += " · " + scan.Iface
			}
		}
		ui.Text(c, label).SingleLine().Grow(1).Shrink(1)
		switch {
		case a.scanning:
			ui.Text(c, "正在刷新").NoWrap()
		case !a.loadedAt.IsZero():
			ui.Text(c, "更新于 "+ago(a.now(), a.loadedAt)).NoWrap()
		}
	})
}

func (a *app) problem(c *ui.Context, k ink, title, detail string, permission bool) {
	ui.Column(c).Grow(1).Center().Gap(6).Padding(24, 32).Children(func() {
		ui.Text(c, title).FontSize(14).FontWeight(600).TextAlign(ui.Center)
		ui.Text(c, detail).FontSize(12).TextColor(k.muted).TextAlign(ui.Center).LineHeight(1.4)
		if permission {
			ui.Text(c, "系统设置 → 隐私与安全性 → 本地网络 → 允许 mylan。").
				FontSize(12).TextColor(k.muted).TextAlign(ui.Center).LineHeight(1.4)
		}
		ui.Row(c).Gap(8).Margin(10, 0, 0, 0).Children(func() {
			ui.Button(c.Key("retry"), "重试").OnClick(a.refresh)
			ui.Button(c.Key("open-settings"), "设置").OnClick(a.openSettings)
		})
	})
}

// copyText puts text on the clipboard and shows a short confirmation.
func (a *app) copyText(label, text string) {
	text = trimCopy(text)
	if text == "" {
		return
	}
	if a.copy != nil {
		a.copy(text)
	} else {
		mygo.Clipboard.WriteText(text)
	}
	a.copied = label
	a.copiedAt = a.now()
}

func trimCopy(s string) string {
	if s == "" || s == "—" || s == "-" {
		return ""
	}
	return s
}
