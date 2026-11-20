package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo"
)

// watchSnap is the last state seen at a monitored IP.
type watchSnap struct {
	Ready  bool   `json:"ready"`
	MAC    string `json:"mac,omitempty"`
	Name   string `json:"name,omitempty"`
	Vendor string `json:"vendor,omitempty"`
	Online bool   `json:"online"`
}

func noteKey(d device) string {
	if mac := d.mac(); mac != "" {
		return "mac:" + mac
	}
	if ip := d.ip(); ip != "" {
		return "ip:" + ip
	}
	return ""
}

func (a *app) note(d device) string {
	if a.cfg.Notes == nil {
		return ""
	}
	if key := noteKey(d); key != "" {
		return a.cfg.Notes[key]
	}
	return ""
}

func (a *app) saveNote(d device) {
	key := noteKey(d)
	if key == "" {
		return
	}
	if a.cfg.Notes == nil {
		a.cfg.Notes = map[string]string{}
	}
	text := strings.TrimSpace(a.editNote)
	if text == "" {
		if _, ok := a.cfg.Notes[key]; !ok {
			return
		}
		delete(a.cfg.Notes, key)
		a.persist()
		return
	}
	if a.cfg.Notes[key] == text {
		return
	}
	a.cfg.Notes[key] = text
	a.persist()
}

func (a *app) watching(ip string) bool {
	if a.cfg.Watches == nil || ip == "" {
		return false
	}
	_, ok := a.cfg.Watches[ip]
	return ok
}

// setWatch starts or stops monitoring an IP. Turning it on records the
// device there now, so the next scan only notifies when something changes.
func (a *app) setWatch(ip string, on bool) {
	if ip == "" {
		return
	}
	if a.cfg.Watches == nil {
		a.cfg.Watches = map[string]watchSnap{}
	}
	if !on {
		if _, ok := a.cfg.Watches[ip]; !ok {
			return
		}
		delete(a.cfg.Watches, ip)
		a.persist()
		return
	}
	snap := watchSnap{}
	for _, d := range a.report.Devices {
		if d.ip() == ip {
			snap = snapOf(d)
			break
		}
	}
	a.cfg.Watches[ip] = snap
	a.persist()
}

func snapOf(d device) watchSnap {
	return watchSnap{
		Ready:  true,
		MAC:    d.mac(),
		Name:   d.name(),
		Vendor: d.Vendor,
		Online: d.Online,
	}
}

// checkWatches compares a new scan with each monitored IP and returns the
// messages worth a notification. The first time an IP is seen only records
// a baseline.
func (a *app) checkWatches(rep report) []string {
	if len(a.cfg.Watches) == 0 {
		return nil
	}
	byIP := map[string]device{}
	for _, d := range rep.Devices {
		if ip := d.ip(); ip != "" {
			byIP[ip] = d
		}
	}
	ips := make([]string, 0, len(a.cfg.Watches))
	for ip := range a.cfg.Watches {
		ips = append(ips, ip)
	}
	slices.Sort(ips)
	var msgs []string
	dirty := false
	for _, ip := range ips {
		prev := a.cfg.Watches[ip]
		d, ok := byIP[ip]
		if !prev.Ready {
			if ok {
				a.cfg.Watches[ip] = snapOf(d)
				dirty = true
			}
			continue
		}
		if !ok {
			if prev.Online {
				msgs = append(msgs, fmt.Sprintf("%s 离线：%s", ip, who(prev)))
				prev.Online = false
				a.cfg.Watches[ip] = prev
				dirty = true
			}
			continue
		}
		cur := snapOf(d)
		if msg := diffWatch(ip, prev, cur); msg != "" {
			msgs = append(msgs, msg)
		}
		if cur != prev {
			a.cfg.Watches[ip] = cur
			dirty = true
		}
	}
	if dirty {
		a.persist()
	}
	return msgs
}

func diffWatch(ip string, prev, cur watchSnap) string {
	if prev.MAC != "" && cur.MAC != "" && prev.MAC != cur.MAC {
		return fmt.Sprintf("%s 设备变了：%s → %s", ip, who(prev), who(cur))
	}
	if prev.Online != cur.Online {
		if cur.Online {
			return fmt.Sprintf("%s 上线：%s", ip, who(cur))
		}
		return fmt.Sprintf("%s 离线：%s", ip, who(prev))
	}
	if prev.Name != cur.Name {
		return fmt.Sprintf("%s 名称从 %s 变为 %s", ip, orUnknown(prev.Name), orUnknown(cur.Name))
	}
	if prev.Vendor != "" && cur.Vendor != "" && prev.Vendor != cur.Vendor {
		return fmt.Sprintf("%s 厂商从 %s 变为 %s", ip, prev.Vendor, cur.Vendor)
	}
	return ""
}

func who(s watchSnap) string {
	if s.Name != "" {
		return s.Name
	}
	if s.MAC != "" {
		return s.MAC
	}
	if s.Vendor != "" {
		return s.Vendor
	}
	return "未知设备"
}

func orUnknown(s string) string {
	if s == "" {
		return "未命名"
	}
	return s
}

func (a *app) notify(body string) {
	if body == "" {
		return
	}
	if a.alert != nil {
		a.alert(body)
		return
	}
	n := mygo.NewNotification(mygo.NotificationOptions{
		Title: "mylan",
		Body:  body,
		Group: "watch",
	})
	// Show waits if the system is asking for notification permission.
	go func() { _ = n.Show() }()
}
