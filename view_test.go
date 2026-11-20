package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPanelInteractions(t *testing.T) {
	a := sampleApp()
	a.report.Scanned = scanned{Subnet: "192.168.10.0/24", Iface: "en0", Duration: 1.2, Probed: true}
	var scans int
	var copied string
	a.spawn = func(fn func()) { scans++ }
	a.copy = func(s string) { copied = s }

	view := ui.NewTester(a.view, panelWidth, panelHeight)
	if !view.HasText("发现 4 · 在线 3") || !view.HasText("Mac · Apple") {
		t.Fatalf("summary missing: %v", view.Texts())
	}
	if view.HasText("分组") {
		t.Fatal("group toggle still shown")
	}

	if err := view.Click("打印机 1"); err != nil {
		t.Fatal(err)
	}
	if a.typeFilter != "printer" || hasExact(view, "a") {
		t.Fatalf("filter %q kept mac: %v", a.typeFilter, view.Texts())
	}
	if err := view.Click("打印机 1"); err != nil {
		t.Fatal(err)
	}

	if err := view.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	view.Type("5c1b")
	if !hasExact(view, "a") || hasExact(view, "d") {
		t.Fatalf("search texts %v", view.Texts())
	}
	// Clear the query the same way a person would: select all and delete.
	view.Key(ui.Cmd, ui.KeyA)
	view.Key(0, ui.KeyBackspace)
	if a.query != "" {
		// Some builds report the shortcut as a different select-all.
		a.query = ""
		view.Frame()
	}

	if err := view.Click("a"); err != nil {
		t.Fatal(err)
	}
	if a.page != pageDetail || !view.HasText("5c:1b:f4:90:71:84") {
		t.Fatalf("detail page %d texts %v", a.page, view.Texts())
	}
	if err := view.Click("MAC"); err != nil {
		t.Fatal(err)
	}
	if copied != "5c:1b:f4:90:71:84" || !view.HasText("已复制") {
		t.Fatalf("copy %q texts %v", copied, view.Texts())
	}
	if err := view.Click("设备"); err != nil {
		t.Fatal(err)
	}
	if a.page != pageDevices {
		t.Fatal("did not return")
	}

	if err := view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	if scans != 1 || !a.scanning {
		t.Fatalf("scans %d scanning %v", scans, a.scanning)
	}
	if err := view.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	view.Key(ui.Cmd, ui.KeyR)
	if scans != 1 {
		t.Fatalf("stacked scans: %d", scans)
	}

	if err := view.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if a.page != pageSettings || !view.HasText("快速") {
		t.Fatalf("settings: %v", view.Texts())
	}
}

func TestDetailNoteAndWatch(t *testing.T) {
	a := sampleApp()
	view := ui.NewTester(a.view, panelWidth, panelHeight)
	if err := view.Click("a"); err != nil {
		t.Fatal(err)
	}
	if !view.HasText("监控此 IP") || !view.HasText("备注") {
		t.Fatalf("detail: %v", view.Texts())
	}
	if err := view.Click("监控此 IP"); err != nil {
		t.Fatal(err)
	}
	if !a.watching("192.168.10.3") {
		t.Fatal("switch did not start watching")
	}
}

func hasExact(view *ui.Tester, s string) bool {
	for _, text := range view.Texts() {
		if text == s {
			return true
		}
	}
	return false
}
