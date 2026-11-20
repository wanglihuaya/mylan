package main

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const version = "0.3.0"

const (
	pageDevices = iota
	pageDetail
	pageSettings
)

// config is what the app remembers. Viewing and filtering of a finished
// scan stay in memory; these fields change what the next scan does.
type config struct {
	Fast          bool                 `json:"fast,omitempty"`
	Stale         bool                 `json:"stale,omitempty"`
	Names         bool                 `json:"names,omitempty"`
	Subnet        string               `json:"subnet,omitempty"`
	AutoRefresh   int                  `json:"autoRefresh,omitempty"` // seconds; 0, 60 or 300
	Sort          string               `json:"sort,omitempty"`        // ip, name, type
	Notes         map[string]string    `json:"notes,omitempty"`
	Watches       map[string]watchSnap `json:"watches,omitempty"`
	HideMenuCount bool                 `json:"hideMenuCount,omitempty"`
}

// app is the panel's state. The view reads it; scans publish through apply.
type app struct {
	cfg        config
	dataDir    string
	query      string
	typeFilter string
	page       int
	selected   string
	editSubnet string
	editNote   string

	report   report
	loadedAt time.Time
	scanning bool
	gen      int
	scanErr  error

	copied   string
	copiedAt time.Time

	post     func(func())
	spawn    func(func())
	changed  func(config)
	setLogin func(bool) error
	login    bool
	loginErr string
	quit     func()
	close    func()
	clock    func() time.Time
	copy     func(string)
	alert    func(string)
}

var typeOrder = []string{
	"mac", "apple", "pc", "linux", "phone", "tablet", "tv", "speaker",
	"printer", "router", "nas", "camera", "iot", "unknown",
}

var typeLabels = map[string]string{
	"mac": "Mac", "apple": "Apple 设备", "pc": "Windows PC", "linux": "Linux",
	"phone": "手机", "tablet": "平板", "tv": "电视/投屏", "speaker": "音箱",
	"printer": "打印机", "router": "路由器", "nas": "NAS", "camera": "摄像头",
	"iot": "智能设备", "unknown": "未知",
}

func loadConfig(dir string) config {
	cfg := config{Sort: "ip"}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	switch cfg.Sort {
	case "ip", "name", "type":
	default:
		cfg.Sort = "ip"
	}
	switch cfg.AutoRefresh {
	case 0, 60, 300:
	default:
		cfg.AutoRefresh = 0
	}
	return cfg
}

func writeConfig(dir string, cfg config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := filepath.Join(dir, "config.json.tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "config.json"))
}

func (a *app) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

func (a *app) persist() {
	if a.changed != nil {
		a.changed(a.cfg)
	}
}

// beginScan starts one scan. A second call while one is running does nothing,
// so refreshes do not stack pings.
func (a *app) beginScan() (int, bool) {
	if a.scanning {
		return 0, false
	}
	a.scanning = true
	a.gen++
	return a.gen, true
}

// apply publishes a scan finished for gen. An older gen is dropped.
func (a *app) apply(gen int, rep report, err error) bool {
	if gen != a.gen {
		return false
	}
	a.scanning = false
	if err != nil {
		a.scanErr = err
		return true
	}
	a.scanErr = nil
	for _, msg := range a.checkWatches(rep) {
		a.notify(msg)
	}
	a.report = rep
	a.loadedAt = a.now()
	return true
}

func (a *app) refresh() {
	gen, ok := a.beginScan()
	if !ok {
		return
	}
	cfg := a.cfg
	dir := a.dataDir
	run := func() {
		rep, err := runScan(dir, cfg)
		done := func() { a.apply(gen, rep, err) }
		if a.post != nil {
			a.post(done)
			return
		}
		done()
	}
	if a.spawn != nil {
		a.spawn(run)
		return
	}
	go run()
}

// stale is true before the first result, and when the last one is over two minutes old.
func (a *app) stale() bool {
	if a.loadedAt.IsZero() {
		return true
	}
	return a.now().Sub(a.loadedAt) > 2*time.Minute
}

func (a *app) onlineCount() int {
	n := 0
	for _, d := range a.report.Devices {
		if d.Online {
			n++
		}
	}
	return n
}

func (a *app) back() {
	if a.page == pageDetail {
		if d, ok := a.selectedDevice(); ok {
			a.saveNote(d)
		}
	}
	if a.page == pageSettings {
		a.cfg.Subnet = strings.TrimSpace(a.editSubnet)
		a.persist()
	}
	a.page = pageDevices
	a.selected = ""
}

func (a *app) openSettings() {
	if a.page != pageSettings {
		a.editSubnet = a.cfg.Subnet
	}
	a.page = pageSettings
}

func (a *app) toggleType(t string) {
	if a.typeFilter == t {
		a.typeFilter = ""
		return
	}
	a.typeFilter = t
}

// searched is the scan filtered by the query only, so type chips keep their counts.
func (a *app) searched() []device {
	q := strings.ToLower(strings.TrimSpace(a.query))
	if q == "" {
		return a.report.Devices
	}
	out := make([]device, 0, len(a.report.Devices))
	for _, d := range a.report.Devices {
		if matches(d, q) || strings.Contains(strings.ToLower(a.note(d)), q) {
			out = append(out, d)
		}
	}
	return out
}

// visible applies the type chip and the current sort.
func (a *app) visible() []device {
	src := a.searched()
	if a.typeFilter != "" {
		out := make([]device, 0, len(src))
		for _, d := range src {
			if d.Type == a.typeFilter {
				out = append(out, d)
			}
		}
		src = out
	}
	return sortDevices(src, a.cfg.Sort)
}

func (a *app) selectedDevice() (device, bool) {
	for _, d := range a.report.Devices {
		if deviceKey(d) == a.selected {
			return d, true
		}
	}
	return device{}, false
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (d device) ip() string { return deref(d.IP) }

func (d device) mac() string { return deref(d.MAC) }

func (d device) name() string { return deref(d.Name) }

func (d device) hostname() string {
	return strings.TrimSuffix(deref(d.Hostname), ".local")
}

func (d device) modelName() string {
	if s := deref(d.ModelName); s != "" {
		return s
	}
	return deref(d.Model)
}

func (d device) typeLabel() string {
	if d.TypeLabel != "" {
		return d.TypeLabel
	}
	if l, ok := typeLabels[d.Type]; ok {
		return l
	}
	if d.Type == "" {
		return "未知"
	}
	return d.Type
}

func (d device) displayName() string {
	if n := d.name(); n != "" {
		return n
	}
	if ip := d.ip(); ip != "" {
		return ip
	}
	return "未命名"
}

func (d device) status() string {
	if d.IsLocal {
		return "本机"
	}
	if d.Online {
		return "在线"
	}
	if d.ARPCached {
		return "ARP缓存"
	}
	if d.ip() != "" {
		return "无响应"
	}
	return "IP 未知"
}

func (d device) portsText() string {
	if len(d.OpenPorts) == 0 {
		return ""
	}
	parts := make([]string, len(d.OpenPorts))
	for i, p := range d.OpenPorts {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ", ")
}

func deviceKey(d device) string {
	if ip := d.ip(); ip != "" {
		return "ip:" + ip
	}
	if mac := d.mac(); mac != "" {
		return "mac:" + mac
	}
	return "name:" + d.name() + "/" + d.Type
}

func matches(d device, q string) bool {
	mac := strings.ReplaceAll(d.mac(), ":", "")
	blob := strings.ToLower(strings.Join([]string{
		d.name(), d.ip(), d.mac(), mac, d.hostname(), d.Vendor,
		d.typeLabel(), d.modelName(), d.Type,
	}, " "))
	return strings.Contains(blob, q)
}

func sortDevices(devs []device, key string) []device {
	out := append([]device(nil), devs...)
	slices.SortStableFunc(out, func(a, b device) int {
		switch key {
		case "name":
			an, bn := a.name(), b.name()
			if an == "" || bn == "" {
				if an == "" && bn == "" {
					return compareIP(a.ip(), b.ip())
				}
				if an == "" {
					return 1
				}
				return -1
			}
			if c := strings.Compare(strings.ToLower(an), strings.ToLower(bn)); c != 0 {
				return c
			}
			return compareIP(a.ip(), b.ip())
		case "type":
			if c := typeIndex(a.Type) - typeIndex(b.Type); c != 0 {
				return c
			}
			return compareIP(a.ip(), b.ip())
		default:
			return compareIP(a.ip(), b.ip())
		}
	})
	return out
}

func typeIndex(t string) int {
	if t == "" {
		t = "unknown"
	}
	i := slices.Index(typeOrder, t)
	if i < 0 {
		return len(typeOrder)
	}
	return i
}

func compareIP(a, b string) int {
	pa, ea := netip.ParseAddr(a)
	pb, eb := netip.ParseAddr(b)
	switch {
	case ea != nil && eb != nil:
		return 0
	case ea != nil:
		return 1
	case eb != nil:
		return -1
	}
	return pa.Compare(pb)
}

func ago(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < 10*time.Second:
		return "刚刚"
	case d < time.Minute:
		return strconv.Itoa(int(d.Seconds())) + " 秒前"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + " 分钟前"
	default:
		return t.Format("15:04")
	}
}
