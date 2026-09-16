package scan

import (
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/mdlayher/arp"
)

// arpResolver sends raw ARP requests from within the process itself (no
// reliance on the host OS's ARP cache or external "arp" binary, so it
// behaves the same regardless of what the container runs on top of) and
// collects replies in the background. It only works when the process has
// L2 access to the target subnet (e.g. macvlan or host networking) and
// NET_RAW/NET_ADMIN capabilities; when it can't open a raw socket it is
// silently a no-op so the rest of discovery still works via ICMP/TCP.
type arpResolver struct {
	client *arp.Client

	mu      sync.Mutex
	results map[string]net.HardwareAddr
}

// newARPResolver finds a local interface whose configured network overlaps
// the target CIDR and opens a raw ARP socket on it. Returns nil, nil if no
// such interface exists or ARP sockets aren't available on this platform.
func newARPResolver(cidr string) *arpResolver {
	_, target, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}

	ifi := findInterfaceFor(target)
	if ifi == nil {
		return nil
	}

	client, err := arp.Dial(ifi)
	if err != nil {
		return nil
	}

	r := &arpResolver{
		client:  client,
		results: make(map[string]net.HardwareAddr),
	}
	go r.listen()
	return r
}

func findInterfaceFor(target *net.IPNet) *net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(ifi.HardwareAddr) == 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			if target.Contains(ipnet.IP) || ipnet.Contains(target.IP) {
				ifi := ifi
				return &ifi
			}
		}
	}
	return nil
}

func (r *arpResolver) listen() {
	for {
		pkt, _, err := r.client.Read()
		if err != nil {
			return // socket closed
		}
		if pkt.Operation != arp.OperationReply {
			continue
		}
		r.mu.Lock()
		r.results[pkt.SenderIP.String()] = pkt.SenderHardwareAddr
		r.mu.Unlock()
	}
}

// request broadcasts an ARP request for ip. Replies are collected
// asynchronously and read back later via lookup.
func (r *arpResolver) request(ip string) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return
	}
	_ = r.client.SetWriteDeadline(time.Now().Add(timeout))
	_ = r.client.Request(addr)
}

func (r *arpResolver) lookup(ip string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if mac, ok := r.results[ip]; ok {
		return mac.String()
	}
	return ""
}

func (r *arpResolver) close() {
	_ = r.client.Close()
}
