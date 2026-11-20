package main

import (
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestHostsOf(t *testing.T) {
	p := netip.MustParsePrefix("192.168.1.0/24")
	hosts, err := hostsOf(p)
	if err != nil || len(hosts) != 254 || hosts[0].String() != "192.168.1.1" || hosts[len(hosts)-1].String() != "192.168.1.254" {
		t.Fatalf("hosts %d err %v first %v", len(hosts), err, hosts)
	}
	if hostInPrefix(p, netip.MustParseAddr("192.168.1.255")) || hostInPrefix(p, netip.MustParseAddr("192.168.1.0")) {
		t.Fatal("network or broadcast was kept")
	}
	if !hostInPrefix(p, netip.MustParseAddr("192.168.1.1")) {
		t.Fatal("host was dropped")
	}
	if _, err := hostsOf(netip.MustParsePrefix("10.0.0.0/16")); err == nil {
		t.Fatal("large prefix was accepted")
	}
}

func TestAssembleWindows(t *testing.T) {
	face := lanFace{
		Name: "Ethernet", LocalIP: netip.MustParseAddr("192.168.1.20"),
		Prefix: netip.MustParsePrefix("192.168.1.0/24"), Gateway: netip.MustParseAddr("192.168.1.1"),
		HostName: "desk",
	}
	gw := netip.MustParseAddr("192.168.1.1")
	pc := netip.MustParseAddr("192.168.1.50")
	sights := []hostSight{
		{IP: gw, MAC: "50:c7:bf:00:00:01", Online: true},
		{IP: pc, MAC: "00:1b:63:00:00:02", Online: true},
		{IP: face.LocalIP, MAC: "00:11:22:33:44:55", Online: true},
	}
	hits := []serviceHit{{
		IP: pc, Name: "Office", Priority: 55, Hostname: "office.local.",
		Labels: []string{"文件共享"}, Types: []string{"_smb._tcp"},
	}}
	rep := assembleWindows(face, sights, hits, nil, map[netip.Addr][]int{pc: {445}}, time.Second, nil)
	if rep.Scanned.Subnet != "192.168.1.0/24" || len(rep.Devices) != 3 {
		t.Fatalf("%+v devices %d", rep.Scanned, len(rep.Devices))
	}
	var found bool
	for _, d := range rep.Devices {
		if d.IP != nil && *d.IP == pc.String() {
			found = true
			if d.Type != "mac" && d.Type != "pc" {
				t.Fatalf("type %s", d.Type)
			}
			if deref(d.Name) != "Office" || d.Vendor != "Apple, Inc." {
				t.Fatalf("name %q vendor %q", deref(d.Name), d.Vendor)
			}
		}
		if d.IP != nil && *d.IP == gw.String() && d.Type != "router" {
			t.Fatalf("gateway type %s", d.Type)
		}
	}
	if !found {
		t.Fatal("missing pc")
	}
}

func TestParseNetBIOS(t *testing.T) {
	q := netbiosQuery()
	name := make([]byte, 16)
	copy(name, "OFFICE")
	for i := len("OFFICE"); i < 15; i++ {
		name[i] = ' '
	}
	data := []byte{1}
	data = append(data, name...)
	data = append(data, 0x04, 0x00)
	pkt := append([]byte{}, q...)
	pkt[2] = 0x80
	pkt[4], pkt[5] = 0, 1
	pkt = append(pkt, 0xC0, 0x0C)
	pkt = append(pkt, 0x00, 0x21, 0x00, 0x01, 0, 0, 0, 0)
	pkt = append(pkt, byte(len(data)>>8), byte(len(data)))
	pkt = append(pkt, data...)
	if got := parseNetBIOS(pkt); got != "OFFICE" {
		t.Fatalf("got %q", got)
	}
}

func TestMDNSQueryAndParse(t *testing.T) {
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{Response: true, Authoritative: true})
	b.EnableCompression()
	if err := b.StartAnswers(); err != nil {
		t.Fatal(err)
	}
	ptrName := dnsmessage.MustNewName("_airplay._tcp.local.")
	inst := dnsmessage.MustNewName("Living Room._airplay._tcp.local.")
	host := dnsmessage.MustNewName("Living-Room.local.")
	hdr := func(name dnsmessage.Name, typ dnsmessage.Type) dnsmessage.ResourceHeader {
		return dnsmessage.ResourceHeader{Name: name, Type: typ, Class: dnsmessage.ClassINET, TTL: 120}
	}
	if err := b.PTRResource(hdr(ptrName, dnsmessage.TypePTR), dnsmessage.PTRResource{PTR: inst}); err != nil {
		t.Fatal(err)
	}
	if err := b.SRVResource(hdr(inst, dnsmessage.TypeSRV), dnsmessage.SRVResource{Port: 7000, Target: host}); err != nil {
		t.Fatal(err)
	}
	if err := b.TXTResource(hdr(inst, dnsmessage.TypeTXT), dnsmessage.TXTResource{TXT: []string{"model=AppleTV6,2"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.AResource(hdr(host, dnsmessage.TypeA), dnsmessage.AResource{A: [4]byte{192, 168, 1, 40}}); err != nil {
		t.Fatal(err)
	}
	raw, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	table := newMDNSTable()
	table.add(raw)
	hits := table.hits()
	if len(hits) != 1 || hits[0].Name != "living room" || hits[0].IP.String() != "192.168.1.40" || !strings.Contains(hits[0].Model, "appletv") {
		t.Fatalf("%+v", hits)
	}
	if classifyHost(false, hits[0].Model, hits[0].Name, hits[0].Hostname, "", hits[0].Types, nil) != "tv" {
		t.Fatal("expected tv")
	}
	pkt, err := mdnsQuery("_smb._tcp.local.", true)
	if err != nil || len(pkt) < 12 {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint16(pkt[4:6]) != 1 {
		t.Fatalf("qdcount %d", binary.BigEndian.Uint16(pkt[4:6]))
	}
}
