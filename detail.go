package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) detail(c *ui.Context, k ink) {
	if a.copied != "" && a.now().Sub(a.copiedAt) < 1200*time.Millisecond {
		c.After(1200 * time.Millisecond)
	}
	ui.Row(c).Padding(12, 12, 4, 8).Children(func() {
		textButton(c.Key("back"), k, "设备", iconBack, true).OnClick(a.back)
	})
	d, ok := a.selectedDevice()
	if !ok {
		a.problem(c, k, "这台设备不在这次扫描里", "它可能已经离开网络。返回列表再刷新一次。", false)
		return
	}
	ui.Scroll(c.Key("detail")).Grow(1).Padding(8, gutter, gutter, gutter).Gap(16).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, d.displayName()).FontSize(17).FontWeight(650).SingleLine().Shrink(1)
				ui.Text(c, d.status()).FontSize(12).TextColor(k.muted).NoWrap()
			})
			meta := d.typeLabel()
			if m := d.modelName(); m != "" {
				meta += " · " + m
			}
			ui.Text(c, meta).FontSize(12).TextColor(k.muted).SingleLine()
		})
		field(c, k, "备注", "记在这台设备上，按 MAC 保存。", func() {
			ui.TextInput(c.Key("note"), &a.editNote).Label("备注").Placeholder("例如：前台打印机").
				OnChange(func() { a.saveNote(d) }).
				OnSubmit(func() { a.saveNote(d) })
		})
		watching := a.watching(d.ip())
		switchRow(c.Key("watch"), k, "监控此 IP", "上线、离线、换成别的设备或名称变化时通知。", &watching, func() {
			a.setWatch(d.ip(), watching)
		})
		ui.Text(c, "点按复制 IP、MAC 或主机名").FontSize(11).TextColor(k.faint)
		ui.Column(c).Gap(2).Children(func() {
			a.copyRow(c.Key("ip"), k, "IP", dash(d.ip()))
			a.copyRow(c.Key("mac"), k, "MAC", dash(d.mac()))
			a.copyRow(c.Key("host"), k, "主机名", dash(d.hostname()))
			a.infoRow(c.Key("vendor"), k, "厂商", dash(d.Vendor))
			if m := d.modelName(); m != "" {
				a.infoRow(c.Key("model"), k, "机型", m)
			}
			ports := d.portsText()
			if ports == "" && !a.report.Scanned.Probed {
				ports = "未探测"
			}
			a.infoRow(c.Key("ports"), k, "开放端口", dash(ports))
			services := strings.Join(nonEmpty(d.Services), "、")
			a.infoRow(c.Key("services"), k, "服务", dash(services))
		})
	})
}

func (a *app) copyRow(c *ui.Context, k ink, label, value string) {
	b := ui.ButtonBase(c).Padding(8, 10).Radius(8).Gap(8).AlignItems(ui.Center).Label("复制" + label)
	switch {
	case b.Pressed():
		b.Background(k.press)
	case b.Hovered():
		b.Background(k.hover)
	}
	b.OnClick(func() { a.copyText(label, value) })
	b.Children(func() {
		a.fieldTexts(c, k, label, value)
		if a.copied == label && a.now().Sub(a.copiedAt) < 1200*time.Millisecond {
			ui.Text(c, "已复制").FontSize(11).TextColor(k.up).NoWrap()
		}
	})
}

func (a *app) infoRow(c *ui.Context, k ink, label, value string) {
	ui.Row(c).Padding(8, 10).Gap(8).AlignItems(ui.Center).Children(func() {
		a.fieldTexts(c, k, label, value)
	})
}

func (a *app) fieldTexts(c *ui.Context, k ink, label, value string) {
	ui.Text(c, label).Width(64).FontSize(12).TextColor(k.muted).NoWrap()
	ui.Text(c, value).Grow(1).Shrink(1).SingleLine().FontFeatures("tnum")
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" && s != "-" {
			out = append(out, s)
		}
	}
	return out
}
