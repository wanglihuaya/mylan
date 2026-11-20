package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func sp(s string) *string { return &s }

func TestParseReport(t *testing.T) {
	raw := []byte(`{
	  "version": "1.1.0",
	  "scanned": {"subnet": "192.168.10.0/24", "iface": "en0", "local_ip": "192.168.10.214",
	    "local_name": "马克布克", "gateway": "192.168.10.1", "duration_s": 6.2, "probed": true},
	  "warnings": [],
	  "devices": [
	    {"name": "xlsd的Mac mini", "type": "mac", "type_label": "Mac", "model": null, "model_name": null,
	     "ip": "192.168.10.59", "hostname": "xlsddeMac-mini.local", "mac": "5c:1b:f4:90:71:84",
	     "vendor": "Apple", "services": ["AirPlay", "SSH"], "open_ports": [22, 445],
	     "online": true, "is_local": false, "is_gateway": false},
	    {"name": null, "type": "printer", "type_label": "打印机", "ip": "192.168.10.79",
	     "hostname": null, "mac": "00:11:22:33:44:55", "vendor": "HP", "services": [],
	     "open_ports": [9100], "online": true}
	  ]
	}`)
	rep, err := parseReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Scanned.Subnet != "192.168.10.0/24" || len(rep.Devices) != 2 {
		t.Fatalf("report: %+v", rep.Scanned)
	}
	if rep.Devices[0].displayName() != "xlsd的Mac mini" || rep.Devices[0].hostname() != "xlsddeMac-mini" {
		t.Fatalf("device: %#v", rep.Devices[0])
	}
	if rep.Devices[1].name() != "" || rep.Devices[1].portsText() != "9100" {
		t.Fatalf("printer: name %q ports %q", rep.Devices[1].name(), rep.Devices[1].portsText())
	}
	// Leading log noise should not hide the object.
	withNoise := append([]byte("dns-sd: ready\n"), raw...)
	if _, err := parseReport(withNoise); err != nil {
		t.Fatal(err)
	}
	// A fast scan leaves gateway and name_source null.
	nullable := []byte(`{"version":"1.1.0","scanned":{"subnet":"192.168.10.0/24","iface":"en0","gateway":null,"duration_s":3.9,"probed":false},"warnings":["已跳过 ping 扫描"],"devices":[{"name":null,"name_source":null,"type":"printer","type_label":"打印机","ip":"192.168.10.25","hostname":"NPI7B8431.local","mac":"c4:13:75:cd:b4:20","vendor":"HP","services":["打印"],"online":true}]}`)
	got, err := parseReport(nullable)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scanned.Gateway != nil || got.Devices[0].hostname() != "NPI7B8431" || got.Devices[0].displayName() != "192.168.10.25" {
		t.Fatalf("nullable: %+v %+v", got.Scanned, got.Devices[0])
	}
}

func TestScanArgs(t *testing.T) {
	got := strings.Join(scanArgs(config{Fast: true, Names: true, Subnet: " 192.168.1.0/24 "}), " ")
	if got != "-j --fast --names -s 192.168.1.0/24" {
		t.Fatal(got)
	}
	if strings.Join(scanArgs(config{}), " ") != "-j" {
		t.Fatal(scanArgs(config{}))
	}
}

func sampleApp() *app {
	return &app{cfg: config{Sort: "ip"}, report: report{Devices: []device{
		{Name: sp("b"), IP: sp("192.168.10.19"), Type: "phone", TypeLabel: "手机", Vendor: "Xiaomi", Online: true},
		{Name: sp("a"), IP: sp("192.168.10.3"), Type: "mac", TypeLabel: "Mac", MAC: sp("5c:1b:f4:90:71:84"), Vendor: "Apple", Online: true},
		{Name: sp("d"), IP: sp("192.168.10.47"), Type: "printer", TypeLabel: "打印机", Vendor: "HP", Online: false},
		{IP: sp("192.168.10.9"), Type: "unknown", TypeLabel: "未知", Online: true},
	}}}
}

func TestSearchTypeAndSort(t *testing.T) {
	a := sampleApp()
	a.query = "192.168.10.3"
	if got := a.visible(); len(got) != 1 || got[0].name() != "a" {
		t.Fatalf("ip search: %d", len(got))
	}
	a.query = "5c1bf4"
	if got := a.visible(); len(got) != 1 || got[0].name() != "a" {
		t.Fatalf("mac search: %+v", names(a.visible()))
	}
	a.query = "打印"
	if got := a.visible(); len(got) != 1 || got[0].Type != "printer" {
		t.Fatalf("label search: %+v", names(got))
	}
	a.query = ""
	a.typeFilter = "printer"
	if got := a.visible(); len(got) != 1 || got[0].name() != "d" {
		t.Fatalf("type: %+v", names(got))
	}
	a.toggleType("printer")
	if a.typeFilter != "" {
		t.Fatal("toggle off")
	}
	a.cfg.Sort = "name"
	got := names(a.visible())
	if strings.Join(got, ",") != "a,b,d," {
		t.Fatalf("name sort: %#v", got)
	}
	a.cfg.Sort = "ip"
	ips := make([]string, 0)
	for _, d := range a.visible() {
		ips = append(ips, d.ip())
	}
	if strings.Join(ips, ",") != "192.168.10.3,192.168.10.9,192.168.10.19,192.168.10.47" {
		t.Fatal(ips)
	}
}

func TestApplyDropsStale(t *testing.T) {
	a := &app{gen: 2, scanning: true, clock: func() time.Time { return time.Unix(100, 0) }}
	old := report{Devices: []device{{Type: "phone", Online: true}}}
	if a.apply(1, old, nil) {
		t.Fatal("stale result applied")
	}
	if !a.scanning || len(a.report.Devices) != 0 {
		t.Fatalf("stale mutated state: scanning %v devices %d", a.scanning, len(a.report.Devices))
	}
	next := report{Devices: []device{{Type: "mac", Online: true}, {Type: "pc", Online: false}}}
	if !a.apply(2, next, nil) {
		t.Fatal("current result dropped")
	}
	if a.scanning || a.onlineCount() != 1 || !a.loadedAt.Equal(time.Unix(100, 0)) {
		t.Fatalf("apply: scanning %v online %d", a.scanning, a.onlineCount())
	}
	if _, ok := a.beginScan(); !ok {
		t.Fatal("begin after finish")
	}
	if _, ok := a.beginScan(); ok {
		t.Fatal("second scan started")
	}
}

func TestLiveFastScan(t *testing.T) {
	if os.Getenv("MYLAN_LIVE") == "" {
		t.Skip()
	}
	rep, err := runScan(t.TempDir(), config{Fast: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Scanned.Subnet == "" || len(rep.Devices) == 0 {
		t.Fatalf("empty scan: %+v devices %d", rep.Scanned, len(rep.Devices))
	}
	t.Logf("%s %d devices, %d warnings", rep.Scanned.Subnet, len(rep.Devices), len(rep.Warnings))
}

func TestScanUnavailable(t *testing.T) {
	if err := scanUnavailable("darwin"); err != nil {
		t.Fatal(err)
	}
	err := scanUnavailable("windows")
	if err == nil || !strings.Contains(err.Error(), "Windows") || strings.Contains(err.Error(), "本地网络") {
		t.Fatal(err)
	}
	err = scanUnavailable("linux")
	if err == nil || !strings.Contains(err.Error(), "Linux") {
		t.Fatal(err)
	}
}

func names(devs []device) []string {
	out := make([]string, len(devs))
	for i, d := range devs {
		out[i] = d.name()
	}
	return out
}
