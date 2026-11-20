package main

import (
	"strings"
	"testing"
)

func TestNoteAndWatch(t *testing.T) {
	a := sampleApp()
	d := a.report.Devices[1]
	a.editNote = "前台"
	a.saveNote(d)
	if a.note(d) != "前台" {
		t.Fatal(a.note(d))
	}
	a.query = "前台"
	if got := a.visible(); len(got) != 1 || got[0].name() != "a" {
		t.Fatalf("note search: %v", names(a.visible()))
	}
	a.query = ""

	var alerts []string
	a.alert = func(s string) { alerts = append(alerts, s) }
	a.setWatch(d.ip(), true)
	if !a.watching(d.ip()) || !a.cfg.Watches[d.ip()].Ready {
		t.Fatal("watch did not record a baseline")
	}
	if msgs := a.checkWatches(a.report); len(msgs) != 0 {
		t.Fatal(msgs)
	}

	changed := a.report
	changed.Devices = append([]device(nil), a.report.Devices...)
	changed.Devices[1].MAC = sp("aa:bb:cc:dd:ee:ff")
	changed.Devices[1].Name = sp("别人的电脑")
	msgs := a.checkWatches(changed)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "设备变了") {
		t.Fatal(msgs)
	}

	msgs = a.checkWatches(report{})
	if len(msgs) != 1 || !strings.Contains(msgs[0], "离线") {
		t.Fatal(msgs)
	}
	if msgs = a.checkWatches(report{}); len(msgs) != 0 {
		t.Fatal("repeated offline", msgs)
	}

	back := a.report
	back.Devices = append([]device(nil), a.report.Devices...)
	msgs = a.checkWatches(back)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "设备变了") {
		t.Fatal(msgs)
	}

	named := back
	named.Devices = append([]device(nil), back.Devices...)
	named.Devices[1].Name = sp("改名了")
	msgs = a.checkWatches(named)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "名称从") {
		t.Fatal(msgs)
	}

	a.gen = 1
	a.scanning = true
	if !a.apply(1, named, nil) {
		t.Fatal("apply")
	}
	if len(alerts) != 0 {
		t.Fatal("same snapshot notified", alerts)
	}
	renamed := named
	renamed.Devices = append([]device(nil), named.Devices...)
	renamed.Devices[1].Name = sp("又改了")
	a.gen = 2
	a.scanning = true
	if !a.apply(2, renamed, nil) || len(alerts) != 1 {
		t.Fatalf("alerts %v", alerts)
	}
}
