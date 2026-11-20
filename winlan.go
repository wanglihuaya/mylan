package main

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/net/dns/dnsmessage"
)

// ipNetRow2 matches MIB_IPNET_ROW2 on 64-bit Windows.
// SOCKADDR_INET is 28 bytes; the following ULONG sits at offset 28 and the
// NET_LUID follows at offset 32. The struct alignment rounds nothing past 88.
type ipNetRow2 struct {
	Address               [28]byte
	InterfaceIndex        uint32
	InterfaceLuid         uint64
	PhysicalAddress       [32]byte
	PhysicalAddressLength uint32
	State                 uint32
	Flags                 uint8
	_                     [3]byte
	ReachabilityTime      uint32
}

type lanFace struct {
	Name     string
	Index    int
	LocalIP  netip.Addr
	LocalMAC string
	Prefix   netip.Prefix
	Gateway  netip.Addr
	HostName string
}

type hostSight struct {
	IP     netip.Addr
	MAC    string
	Online bool
	Cached bool
}

type serviceHit struct {
	IP       netip.Addr
	Name     string
	Priority int
	Hostname string
	Model    string
	Labels   []string
	Types    []string
}

type winService struct {
	name  string
	label string
	pri   int
}

var winServices = []winService{
	{"_airplay._tcp.local.", "AirPlay", 100},
	{"_companion-link._tcp.local.", "接力", 90},
	{"_googlecast._tcp.local.", "投屏", 70},
	{"_ssh._tcp.local.", "SSH", 60},
	{"_smb._tcp.local.", "文件共享", 55},
	{"_rfb._tcp.local.", "屏幕共享", 50},
	{"_hap._tcp.local.", "HomeKit", 40},
	{"_ipp._tcp.local.", "打印", 20},
	{"_printer._tcp.local.", "打印", 20},
}

var ouiVendors = map[string]string{
	"001B63": "Apple, Inc.",
	"3C0754": "Apple, Inc.",
	"F4289D": "Apple, Inc.",
	"A45E60": "Apple, Inc.",
	"B827EB": "Raspberry Pi Foundation",
	"240AC4": "Espressif Inc.",
	"50C7BF": "TP-LINK TECHNOLOGIES",
	"000E58": "Sonos, Inc.",
	"001A11": "Google, Inc.",
	"F4F5D8": "Google, Inc.",
}

var vendorTypes = []struct {
	keys []string
	typ  string
}{
	{[]string{"synology", "qnap", "buffalo", "drobo", "western digital"}, "nas"},
	{[]string{"hikvision", "dahua", "uniview", "reolink", "ezviz", "axis communications"}, "camera"},
	{[]string{"seiko epson", "brother industries", "ricoh", "sharp corporation", "kyocera", "xerox", "lexmark", "canon inc"}, "printer"},
	{[]string{"sonos", "bose", "harman", "denon", "yamaha"}, "speaker"},
	{[]string{"espressif", "tuya", "broadlink", "shelly", "signify", "philips lighting"}, "iot"},
	{[]string{"tp-link", "tplink", "d-link", "tenda", "ubiquiti", "aruba", "meraki", "mikrotik", "ruijie", "h3c", "linksys", "netgear", "ruckus", "zyxel"}, "router"},
	{[]string{"raspberry pi"}, "linux"},
	{[]string{"google", "roku", "vizio", "sony", "tcl", "hisense", "skyworth"}, "tv"},
	{[]string{"xiaomi", "oppo", "vivo", "oneplus", "motorola", "nokia", "honor", "realme", "meizu", "samsung electronics"}, "phone"},
	{[]string{"dell", "lenovo", "acer", "asustek", "micro-star", "gigabyte", "asrock", "intel corporate", "foxconn", "pegatron", "compal"}, "pc"},
}

func init() {
	if unsafe.Sizeof(ipNetRow2{}) != 88 {
		panic(fmt.Sprintf("MIB_IPNET_ROW2 size is %d, want 88", unsafe.Sizeof(ipNetRow2{})))
	}
}

// hostsOf lists the usable addresses of a prefix. Prefixes larger than /22
// are refused so a scan cannot ping an entire private /8.
func hostsOf(p netip.Prefix) ([]netip.Addr, error) {
	if !p.IsValid() || !p.Addr().Is4() {
		return nil, fmt.Errorf("网段无效")
	}
	p = p.Masked()
	bits := p.Bits()
	if bits < 22 {
		return nil, fmt.Errorf("网段 %s 太大，请在设置里指定，例如 192.168.1.0/24", p)
	}
	n := 1 << (32 - bits)
	out := make([]netip.Addr, 0, n)
	ip := p.Addr()
	for i := range n {
		if bits < 31 && (i == 0 || i == n-1) {
			ip = ip.Next()
			continue
		}
		out = append(out, ip)
		ip = ip.Next()
	}
	return out, nil
}

func hostInPrefix(prefix netip.Prefix, ip netip.Addr) bool {
	if !ip.IsValid() || !prefix.Contains(ip) || ip.IsUnspecified() {
		return false
	}
	bits := prefix.Bits()
	if bits < 0 || bits >= 31 {
		return true
	}
	network := prefix.Masked().Addr()
	if ip == network {
		return false
	}
	raw := network.As4()
	base := binary.BigEndian.Uint32(raw[:])
	var bcast [4]byte
	binary.BigEndian.PutUint32(bcast[:], base|(^uint32(0)>>bits))
	return ip != netip.AddrFrom4(bcast)
}

func parseSubnet(s string) (netip.Prefix, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "/") {
		s += "/32"
	}
	p, err := netip.ParsePrefix(s)
	if err != nil || !p.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("网段无效：%s", strings.TrimSpace(s))
	}
	return p.Masked(), nil
}

func vendorOf(mac string) string {
	hex := strings.ReplaceAll(strings.ToUpper(mac), ":", "")
	if len(hex) < 6 {
		return ""
	}
	return ouiVendors[hex[:6]]
}

func classifyHost(gateway bool, model, name, host, vendor string, services []string, ports []int) string {
	model, name, host, vendor = strings.ToLower(model), strings.ToLower(name), strings.ToLower(host), strings.ToLower(vendor)
	blob := name + " " + host + " " + model
	has := func(port int) bool { return slices.Contains(ports, port) }
	svc := func(s string) bool { return slices.Contains(services, s) }
	if gateway {
		return "router"
	}
	if strings.Contains(model, "macbook") || strings.Contains(model, "imac") || strings.Contains(model, "macmini") || strings.Contains(model, "macpro") || strings.Contains(model, "macstudio") {
		return "mac"
	}
	if strings.Contains(blob, "macbook") || strings.Contains(blob, "mac mini") || strings.Contains(blob, "imac") || strings.Contains(blob, "mac studio") || strings.Contains(blob, "mac pro") {
		return "mac"
	}
	if strings.Contains(model, "audioaccessory") || strings.Contains(blob, "homepod") {
		return "speaker"
	}
	if strings.Contains(model, "appletv") || strings.Contains(blob, "apple tv") {
		return "tv"
	}
	if strings.Contains(model, "iphone") || strings.Contains(blob, "iphone") {
		return "phone"
	}
	if strings.Contains(model, "ipad") || strings.Contains(blob, "ipad") {
		return "tablet"
	}
	if svc("_ipp._tcp") || svc("_printer._tcp") {
		return "printer"
	}
	if svc("_googlecast._tcp") {
		return "tv"
	}
	if svc("_hap._tcp") {
		return "iot"
	}
	for _, word := range []string{"printer", "laserjet", "epson", "canon", "brother", "xerox"} {
		if strings.Contains(blob, word) {
			return "printer"
		}
	}
	for _, word := range []string{"router", "gateway", "access point", "unifi", "openwrt"} {
		if strings.Contains(blob, word) {
			return "router"
		}
	}
	for _, word := range []string{"diskstation", "synology", "qnap"} {
		if strings.Contains(blob, word) {
			return "nas"
		}
	}
	if has(62078) {
		return "phone"
	}
	if has(9100) || has(631) {
		return "printer"
	}
	if has(3389) {
		return "pc"
	}
	if has(5000) || has(5001) {
		return "nas"
	}
	if strings.HasPrefix(vendor, "apple") {
		if has(445) || has(5900) || has(22) || has(7000) {
			return "mac"
		}
		return "apple"
	}
	for _, hint := range vendorTypes {
		for _, key := range hint.keys {
			if strings.Contains(vendor, key) {
				return hint.typ
			}
		}
	}
	if has(445) {
		return "pc"
	}
	if has(22) {
		return "linux"
	}
	return "unknown"
}

func mergeSights(face lanFace, arp map[netip.Addr]string, online map[netip.Addr]struct{}, hits []serviceHit, fast, stale bool) []hostSight {
	seen := map[netip.Addr]hostSight{}
	put := func(s hostSight) {
		if !hostInPrefix(face.Prefix, s.IP) || s.MAC == "ff:ff:ff:ff:ff:ff" {
			return
		}
		old, ok := seen[s.IP]
		if !ok {
			seen[s.IP] = s
			return
		}
		if old.MAC == "" {
			old.MAC = s.MAC
		}
		old.Online = old.Online || s.Online
		old.Cached = old.Cached || s.Cached
		if old.Online {
			old.Cached = false
		}
		seen[s.IP] = old
	}
	for ip, mac := range arp {
		_, up := online[ip]
		switch {
		case fast:
			put(hostSight{IP: ip, MAC: mac, Online: true})
		case up:
			put(hostSight{IP: ip, MAC: mac, Online: true})
		case stale:
			put(hostSight{IP: ip, MAC: mac, Cached: true})
		}
	}
	for ip := range online {
		put(hostSight{IP: ip, MAC: arp[ip], Online: true})
	}
	for _, hit := range hits {
		put(hostSight{IP: hit.IP, MAC: arp[hit.IP], Online: true})
	}
	if face.LocalIP.IsValid() {
		put(hostSight{IP: face.LocalIP, MAC: face.LocalMAC, Online: true})
	}
	out := make([]hostSight, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b hostSight) int { return a.IP.Compare(b.IP) })
	return out
}

func assembleWindows(face lanFace, sights []hostSight, hits []serviceHit, bios map[netip.Addr]string, ports map[netip.Addr][]int, dur time.Duration, warnings []string) report {
	byIP := map[netip.Addr]serviceHit{}
	for _, hit := range hits {
		old, ok := byIP[hit.IP]
		if !ok || hit.Priority > old.Priority {
			if ok {
				hit.Labels = append(append([]string{}, old.Labels...), hit.Labels...)
				hit.Types = append(append([]string{}, old.Types...), hit.Types...)
			}
			byIP[hit.IP] = hit
		} else {
			old.Labels = append(old.Labels, hit.Labels...)
			old.Types = append(old.Types, hit.Types...)
			byIP[hit.IP] = old
		}
	}
	rep := report{
		Version:  "winlan",
		Warnings: warnings,
		Scanned: scanned{
			Subnet:    face.Prefix.String(),
			Iface:     face.Name,
			LocalIP:   face.LocalIP.String(),
			LocalName: face.HostName,
			Duration:  dur.Seconds(),
			Probed:    ports != nil,
			OUISource: "builtin",
		},
	}
	if face.Gateway.IsValid() {
		gw := face.Gateway.String()
		rep.Scanned.Gateway = &gw
	}
	bonjour := map[netip.Addr]struct{}{}
	for _, s := range sights {
		hit := byIP[s.IP]
		name, source, pri := hit.Name, "", hit.Priority
		if name != "" {
			source = "mdns"
		}
		if bios[s.IP] != "" && (name == "" || pri < 50) {
			name, source = bios[s.IP], "netbios"
		}
		if name == "" && s.IP == face.LocalIP {
			name, source = face.HostName, "hostname"
		}
		host := strings.TrimSuffix(hit.Hostname, ".")
		vendor := vendorOf(s.MAC)
		typ := classifyHost(s.IP == face.Gateway, hit.Model, name, host, vendor, hit.Types, ports[s.IP])
		if typ == "unknown" && source == "netbios" {
			typ = "pc"
		}
		d := device{
			Name:         strPtr(name),
			NameSource:   strPtr(source),
			Type:         typ,
			TypeLabel:    typeLabels[typ],
			Model:        strPtr(hit.Model),
			IP:           strPtr(s.IP.String()),
			Hostname:     strPtr(host),
			MAC:          strPtr(s.MAC),
			Vendor:       vendor,
			Services:     uniqueStrings(hit.Labels),
			ServiceTypes: uniqueStrings(hit.Types),
			Online:       s.Online,
			ARPCached:    s.Cached && !s.Online,
			IsLocal:      s.IP == face.LocalIP,
			IsGateway:    s.IP == face.Gateway,
		}
		if ps := ports[s.IP]; len(ps) > 0 {
			d.OpenPorts = append([]int{}, ps...)
			slices.Sort(d.OpenPorts)
			d.Ports = map[string]bool{}
			for _, p := range d.OpenPorts {
				d.Ports[fmt.Sprint(p)] = true
			}
		}
		if len(d.Services) > 0 {
			bonjour[s.IP] = struct{}{}
		}
		if d.Online {
			rep.Scanned.AliveHosts++
		}
		rep.Devices = append(rep.Devices, d)
	}
	rep.Scanned.Bonjour = len(bonjour)
	return rep
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func macString(b []byte) string {
	if len(b) < 6 {
		return ""
	}
	zero := true
	for _, x := range b[:6] {
		if x != 0 {
			zero = false
			break
		}
	}
	if zero {
		return ""
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
}

func ipv4FromSockaddr(b []byte) netip.Addr {
	if len(b) < 8 || binary.LittleEndian.Uint16(b[:2]) != 2 {
		return netip.Addr{}
	}
	return netip.AddrFrom4([4]byte{b[4], b[5], b[6], b[7]})
}

type mdnsTable struct {
	inst map[string]*mdnsDev
	host map[string][]netip.Addr
}

type mdnsDev struct {
	name   string
	pri    int
	host   string
	model  string
	labels []string
	types  []string
}

func newMDNSTable() *mdnsTable {
	return &mdnsTable{inst: map[string]*mdnsDev{}, host: map[string][]netip.Addr{}}
}

func (t *mdnsTable) add(pkt []byte) {
	var msg dnsmessage.Message
	if err := msg.Unpack(pkt); err != nil {
		return
	}
	for _, rr := range append(append([]dnsmessage.Resource{}, msg.Answers...), msg.Additionals...) {
		owner := strings.ToLower(rr.Header.Name.String())
		switch body := rr.Body.(type) {
		case *dnsmessage.PTRResource:
			inst := strings.ToLower(body.PTR.String())
			svc, ok := serviceByName(owner)
			if !ok {
				svc, ok = serviceByName(inst)
			}
			if !ok {
				continue
			}
			dev := t.device(trimSuffixFold(inst, svc.name))
			dev.pri = max(dev.pri, svc.pri)
			if dev.name == "" {
				dev.name = instanceLabel(inst, svc.name)
			}
			dev.labels = append(dev.labels, svc.label)
			dev.types = append(dev.types, strings.TrimSuffix(svc.name, ".local."))
		case *dnsmessage.SRVResource:
			dev := t.device(trimInstance(owner))
			dev.host = strings.ToLower(body.Target.String())
		case *dnsmessage.TXTResource:
			dev := t.device(trimInstance(owner))
			for _, txt := range body.TXT {
				if v, ok := strings.CutPrefix(strings.ToLower(txt), "model="); ok && v != "" {
					dev.model = v
				}
			}
		case *dnsmessage.AResource:
			ip := netip.AddrFrom4(body.A)
			t.host[owner] = append(t.host[owner], ip)
		}
	}
}

func (t *mdnsTable) device(key string) *mdnsDev {
	key = strings.ToLower(strings.TrimSuffix(key, "."))
	if key == "" {
		key = "_"
	}
	dev := t.inst[key]
	if dev == nil {
		dev = &mdnsDev{}
		t.inst[key] = dev
	}
	return dev
}

func (t *mdnsTable) hits() []serviceHit {
	var out []serviceHit
	for _, dev := range t.inst {
		if dev.name == "" && dev.host == "" {
			continue
		}
		ips := t.host[dev.host]
		for _, ip := range ips {
			if !ip.Is4() {
				continue
			}
			out = append(out, serviceHit{
				IP: ip, Name: dev.name, Priority: dev.pri, Hostname: dev.host,
				Model: dev.model, Labels: uniqueStrings(dev.labels), Types: uniqueStrings(dev.types),
			})
		}
	}
	return out
}

func serviceByName(name string) (winService, bool) {
	name = strings.ToLower(name)
	for _, svc := range winServices {
		if strings.Contains(name, strings.TrimSuffix(svc.name, ".local.")) {
			return svc, true
		}
	}
	return winService{}, false
}

func instanceLabel(instance, service string) string {
	base := strings.TrimSuffix(strings.ToLower(instance), ".")
	suf := strings.TrimSuffix(strings.ToLower(service), ".")
	base = strings.TrimSuffix(base, "."+suf)
	base = strings.TrimSuffix(base, suf)
	base = strings.Trim(base, ".")
	if base == "" {
		return ""
	}
	return base
}

func trimSuffixFold(s, suf string) string {
	s = strings.TrimSuffix(strings.ToLower(s), ".")
	suf = strings.TrimSuffix(strings.ToLower(suf), ".")
	return strings.TrimSuffix(s, "."+suf)
}

func trimInstance(owner string) string {
	owner = strings.ToLower(owner)
	for _, svc := range winServices {
		if strings.HasSuffix(strings.TrimSuffix(owner, "."), strings.TrimSuffix(svc.name, ".")) {
			return trimSuffixFold(owner, svc.name)
		}
	}
	return strings.TrimSuffix(owner, ".")
}

func mdnsQuery(service string, unicast bool) ([]byte, error) {
	name, err := dnsmessage.NewName(service)
	if err != nil {
		return nil, err
	}
	class := dnsmessage.ClassINET
	if unicast {
		class |= 0x8000
	}
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	if err := b.Question(dnsmessage.Question{Name: name, Type: dnsmessage.TypePTR, Class: class}); err != nil {
		return nil, err
	}
	return b.Finish()
}

func netbiosQuery() []byte {
	var name [16]byte
	name[0] = '*'
	pkt := []byte{0x13, 0x37, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	pkt = append(pkt, encodeNetBIOS(name)...)
	return append(pkt, 0x00, 0x21, 0x00, 0x01)
}

func encodeNetBIOS(name [16]byte) []byte {
	out := make([]byte, 0, 34)
	out = append(out, 32)
	for _, c := range name {
		out = append(out, 'A'+c>>4, 'A'+c&0x0F)
	}
	return append(out, 0)
}

func parseNetBIOS(pkt []byte) string {
	if len(pkt) < 56 || pkt[2]&0x80 == 0 {
		return ""
	}
	i := skipDNSName(pkt, 12)
	if i+4 > len(pkt) {
		return ""
	}
	i += 4
	i = skipDNSName(pkt, i)
	if i+10 > len(pkt) {
		return ""
	}
	typ := binary.BigEndian.Uint16(pkt[i:])
	i += 8
	rdlen := int(binary.BigEndian.Uint16(pkt[i:]))
	i += 2
	if typ != 0x21 || rdlen < 1 || i+rdlen > len(pkt) {
		return ""
	}
	data := pkt[i : i+rdlen]
	n := int(data[0])
	data = data[1:]
	fallback := ""
	for range n {
		if len(data) < 18 {
			break
		}
		raw := data[:16]
		flags := binary.BigEndian.Uint16(data[16:18])
		data = data[18:]
		if flags&0x8000 != 0 {
			continue
		}
		label := strings.TrimRight(string(raw[:15]), " \x00")
		if label == "" || label == "*" {
			continue
		}
		if raw[15] == 0x00 {
			return label
		}
		if fallback == "" && raw[15] == 0x20 {
			fallback = label
		}
	}
	return fallback
}

func skipDNSName(b []byte, i int) int {
	for i < len(b) {
		n := int(b[i])
		if n == 0 {
			return i + 1
		}
		if n&0xC0 == 0xC0 {
			return i + 2
		}
		if i+1+n > len(b) {
			return len(b)
		}
		i += 1 + n
	}
	return i
}
