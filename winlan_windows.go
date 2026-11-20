//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/net/ipv4"
	"golang.org/x/sys/windows"
)

var (
	iphlpapi       = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreate = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpClose  = iphlpapi.NewProc("IcmpCloseHandle")
	procIcmpEcho   = iphlpapi.NewProc("IcmpSendEcho")
	procNetTable   = iphlpapi.NewProc("GetIpNetTable2")
)

const (
	nlnsProbe      = 2
	nlnsDelay      = 3
	nlnsStale      = 4
	nlnsReachable  = 5
	nlnsPermanent  = 6
	icmpEchoReplyN = 128
)

var probePorts = []int{22, 445, 631, 3389, 5000, 5001, 5900, 9100, 62078}

func scanWindows(ctx context.Context, cfg config) (report, error) {
	start := time.Now()
	face, err := localLAN(strings.TrimSpace(cfg.Subnet))
	if err != nil {
		return report{}, err
	}
	targets, err := hostsOf(face.Prefix)
	if err != nil {
		return report{}, err
	}
	var warnings []string
	arp, arpErr := readNeighbors(face.Prefix)
	online := map[netip.Addr]struct{}{}
	var hits []serviceHit
	var mdnsErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if cfg.Fast {
			return
		}
		online = pingHosts(ctx, targets)
	}()
	go func() {
		defer wg.Done()
		wait := 2500 * time.Millisecond
		if cfg.Fast {
			wait = 1500 * time.Millisecond
		}
		hits, mdnsErr = browseMDNS(ctx, face, wait)
	}()
	wg.Wait()
	if ctx.Err() != nil {
		return report{}, fmt.Errorf("扫描超时")
	}
	if again, err := readNeighbors(face.Prefix); err == nil {
		if arp == nil {
			arp = again
		} else {
			for ip, mac := range again {
				arp[ip] = mac
			}
		}
		arpErr = nil
	}
	if arpErr != nil {
		warnings = append(warnings, "读不到邻居表，MAC 可能不全")
	}
	if cfg.Fast {
		warnings = append(warnings, "已跳过 ping 扫描，在线状态按 ARP 缓存判断")
	}
	if mdnsErr != nil {
		warnings = append(warnings, "mDNS 发现没有完成")
	}
	sights := mergeSights(face, arp, online, hits, cfg.Fast, cfg.Stale)
	bios := map[netip.Addr]string{}
	if cfg.Names {
		bios = netbiosNames(ctx, sights, hits)
	}
	var ports map[netip.Addr][]int
	if !cfg.Fast {
		ports = probeOpen(ctx, sights)
	}
	return assembleWindows(face, sights, hits, bios, ports, time.Since(start), warnings), nil
}

func localLAN(want string) (lanFace, error) {
	var prefix netip.Prefix
	if want != "" {
		p, err := parseSubnet(want)
		if err != nil {
			return lanFace{}, err
		}
		prefix = p
	}
	adapters, err := ipv4Adapters()
	if err != nil {
		return lanFace{}, err
	}
	if want != "" {
		for _, a := range adapters {
			if prefix.Contains(a.LocalIP) {
				a.Prefix = prefix
				return a, nil
			}
		}
		if len(adapters) == 0 {
			return lanFace{}, fmt.Errorf("没有找到可用的局域网网卡")
		}
		adapters[0].Prefix = prefix
		return adapters[0], nil
	}
	if len(adapters) == 0 {
		return lanFace{}, fmt.Errorf("没有找到可用的局域网网卡。请在设置里指定网段，例如 192.168.1.0/24")
	}
	for _, a := range adapters {
		if a.Gateway.IsValid() {
			return a, nil
		}
	}
	return adapters[0], nil
}

func ipv4Adapters() ([]lanFace, error) {
	size := uint32(16 << 10)
	var buf []byte
	var err error
	for range 3 {
		buf = make([]byte, size)
		err = windows.GetAdaptersAddresses(windows.AF_INET, windows.GAA_FLAG_INCLUDE_PREFIX|windows.GAA_FLAG_INCLUDE_GATEWAYS, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if err == nil {
			break
		}
		if err != windows.ERROR_BUFFER_OVERFLOW {
			return nil, fmt.Errorf("读取网卡失败")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("读取网卡失败")
	}
	host, _ := os.Hostname()
	var out []lanFace
	for p := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); p != nil; p = p.Next {
		if p.OperStatus != windows.IfOperStatusUp || p.IfType == 24 {
			continue
		}
		for u := p.FirstUnicastAddress; u != nil; u = u.Next {
			ip4 := u.Address.IP().To4()
			if ip4 == nil {
				continue
			}
			addr, ok := netip.AddrFromSlice(ip4)
			if !ok || !addr.Is4() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || !addr.IsPrivate() {
				continue
			}
			bits := int(u.OnLinkPrefixLength)
			if bits <= 0 || bits > 32 {
				bits = 24
			}
			pref, err := addr.Prefix(bits)
			if err != nil {
				continue
			}
			face := lanFace{
				Name:     windows.UTF16PtrToString(p.FriendlyName),
				Index:    int(p.IfIndex),
				LocalIP:  addr,
				LocalMAC: macString(p.PhysicalAddress[:p.PhysicalAddressLength]),
				Prefix:   pref.Masked(),
				Gateway:  firstGateway(p),
				HostName: host,
			}
			if !face.Gateway.IsValid() {
				face.Gateway = routeGateway(p.IfIndex)
			}
			out = append(out, face)
		}
	}
	return out, nil
}

func firstGateway(p *windows.IpAdapterAddresses) netip.Addr {
	for g := p.FirstGatewayAddress; g != nil; g = g.Next {
		ip4 := g.Address.IP().To4()
		if ip4 == nil {
			continue
		}
		addr, ok := netip.AddrFromSlice(ip4)
		if ok && addr.Is4() && !addr.IsUnspecified() {
			return addr
		}
	}
	return netip.Addr{}
}

func routeGateway(ifIndex uint32) netip.Addr {
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_INET, &table); err != nil || table == nil {
		return netip.Addr{}
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	var best netip.Addr
	bestMetric := uint32(1<<32 - 1)
	for _, row := range table.Rows() {
		if row.DestinationPrefix.PrefixLength != 0 || row.InterfaceIndex != ifIndex {
			continue
		}
		if row.Metric >= bestMetric {
			continue
		}
		ip := ipv4FromSockaddr((*[28]byte)(unsafe.Pointer(&row.NextHop))[:])
		if !ip.IsValid() || ip.IsUnspecified() {
			continue
		}
		best, bestMetric = ip, row.Metric
	}
	return best
}

func readNeighbors(prefix netip.Prefix) (map[netip.Addr]string, error) {
	var table uintptr
	r, _, _ := procNetTable.Call(uintptr(windows.AF_INET), uintptr(unsafe.Pointer(&table)))
	if r != 0 || table == 0 {
		return nil, fmt.Errorf("GetIpNetTable2 %d", r)
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	num := *(*uint32)(unsafe.Pointer(table))
	if num == 0 {
		return map[netip.Addr]string{}, nil
	}
	raw := unsafe.Slice((*ipNetRow2)(unsafe.Add(unsafe.Pointer(table), 8)), num)
	out := make(map[netip.Addr]string, num)
	for _, row := range raw {
		switch row.State {
		case nlnsProbe, nlnsDelay, nlnsStale, nlnsReachable, nlnsPermanent:
		default:
			continue
		}
		ip := ipv4FromSockaddr(row.Address[:])
		if !ip.IsValid() || !prefix.Contains(ip) {
			continue
		}
		n := row.PhysicalAddressLength
		if n > 32 {
			n = 6
		}
		if mac := macString(row.PhysicalAddress[:n]); mac != "" && mac != "ff:ff:ff:ff:ff:ff" {
			out[ip] = mac
		}
	}
	return out, nil
}

func pingHosts(ctx context.Context, ips []netip.Addr) map[netip.Addr]struct{} {
	const workers = 32
	handles := make([]uintptr, 0, workers)
	for range workers {
		h, _, _ := procIcmpCreate.Call()
		if h == 0 || h == ^uintptr(0) {
			continue
		}
		handles = append(handles, h)
	}
	defer func() {
		for _, h := range handles {
			procIcmpClose.Call(h)
		}
	}()
	out := map[netip.Addr]struct{}{}
	if len(handles) == 0 || len(ips) == 0 {
		return out
	}
	jobs := make(chan netip.Addr)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, h := range handles {
		wg.Add(1)
		go func(h uintptr) {
			defer wg.Done()
			for ip := range jobs {
				if ctx.Err() != nil {
					continue
				}
				if pingOne(h, ip, 350*time.Millisecond) {
					mu.Lock()
					out[ip] = struct{}{}
					mu.Unlock()
				}
			}
		}(h)
	}
	for _, ip := range ips {
		if ctx.Err() != nil {
			break
		}
		jobs <- ip
	}
	close(jobs)
	wg.Wait()
	return out
}

func pingOne(handle uintptr, ip netip.Addr, timeout time.Duration) bool {
	b := ip.As4()
	dest := binary.LittleEndian.Uint32(b[:])
	payload := []byte("mylan")
	reply := make([]byte, icmpEchoReplyN)
	r, _, _ := procIcmpEcho.Call(
		handle,
		uintptr(dest),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0,
		uintptr(unsafe.Pointer(&reply[0])),
		uintptr(len(reply)),
		uintptr(timeout.Milliseconds()),
	)
	if r == 0 || len(reply) < 8 {
		return false
	}
	return binary.LittleEndian.Uint32(reply[4:8]) == 0
}

func browseMDNS(ctx context.Context, face lanFace, wait time.Duration) ([]serviceHit, error) {
	conn, unicast, err := listenMDNS(face.Index)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	dst := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	for _, svc := range winServices {
		pkt, err := mdnsQuery(svc.name, unicast)
		if err != nil {
			continue
		}
		_, _ = conn.WriteTo(pkt, dst)
	}
	deadline := time.Now().Add(wait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	table := newMDNSTable()
	buf := make([]byte, 1500)
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	asked := false
	for {
		if time.Now().After(deadline) || ctx.Err() != nil {
			break
		}
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if time.Now().After(deadline) {
				break
			}
			_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			continue
		}
		table.add(append([]byte(nil), buf[:n]...))
		if !asked && time.Until(deadline) < wait/2 {
			asked = true
			for _, svc := range winServices {
				pkt, err := mdnsQuery(svc.name, unicast)
				if err == nil {
					_, _ = conn.WriteTo(pkt, dst)
				}
			}
		}
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	}
	return table.hits(), nil
}

func listenMDNS(ifIndex int) (*net.UDPConn, bool, error) {
	conn, err := listenReuse(5353)
	unicast := false
	if err != nil {
		var err2 error
		conn, err2 = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
		if err2 != nil {
			return nil, false, err
		}
		unicast = true
	}
	p := ipv4.NewPacketConn(conn)
	var iface *net.Interface
	if ifIndex > 0 {
		iface, _ = net.InterfaceByIndex(ifIndex)
	}
	_ = p.JoinGroup(iface, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251)})
	_ = p.SetMulticastTTL(255)
	return conn, unicast, nil
}

func listenReuse(port int) (*net.UDPConn, error) {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) {
			serr = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
		}); err != nil {
			return err
		}
		return serr
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, ok := pc.(*net.UDPConn)
	if !ok {
		pc.Close()
		return nil, fmt.Errorf("udp listener")
	}
	return conn, nil
}

func netbiosNames(ctx context.Context, sights []hostSight, hits []serviceHit) map[netip.Addr]string {
	named := map[netip.Addr]struct{}{}
	for _, hit := range hits {
		if hit.Name != "" {
			named[hit.IP] = struct{}{}
		}
	}
	var todo []netip.Addr
	for _, s := range sights {
		if s.Online && s.IP.IsValid() {
			if _, ok := named[s.IP]; !ok {
				todo = append(todo, s.IP)
			}
		}
	}
	out := map[netip.Addr]string{}
	if len(todo) == 0 {
		return out
	}
	jobs := make(chan netip.Addr)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range min(16, len(todo)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				if ctx.Err() != nil {
					continue
				}
				if name := netbiosName(ip); name != "" {
					mu.Lock()
					out[ip] = name
					mu.Unlock()
				}
			}
		}()
	}
	for _, ip := range todo {
		jobs <- ip
	}
	close(jobs)
	wg.Wait()
	return out
}

func netbiosName(ip netip.Addr) string {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: ip.AsSlice(), Port: 137})
	if err != nil {
		return ""
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(400 * time.Millisecond))
	if _, err := conn.Write(netbiosQuery()); err != nil {
		return ""
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return ""
	}
	return parseNetBIOS(buf[:n])
}

func probeOpen(ctx context.Context, sights []hostSight) map[netip.Addr][]int {
	var ips []netip.Addr
	for _, s := range sights {
		if s.Online {
			ips = append(ips, s.IP)
		}
	}
	out := map[netip.Addr][]int{}
	if len(ips) == 0 {
		return out
	}
	type job struct {
		ip   netip.Addr
		port int
	}
	jobs := make(chan job)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range min(48, len(ips)*len(probePorts)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ctx.Err() != nil {
					continue
				}
				conn, err := net.DialTimeout("tcp", net.JoinHostPort(j.ip.String(), fmt.Sprint(j.port)), 200*time.Millisecond)
				if err != nil {
					continue
				}
				conn.Close()
				mu.Lock()
				out[j.ip] = append(out[j.ip], j.port)
				mu.Unlock()
			}
		}()
	}
	for _, ip := range ips {
		for _, port := range probePorts {
			if ctx.Err() != nil {
				break
			}
			jobs <- job{ip, port}
		}
	}
	close(jobs)
	wg.Wait()
	return out
}
