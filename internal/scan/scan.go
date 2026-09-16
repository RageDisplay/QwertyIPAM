package scan

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

type Host struct {
	IP       string
	Alive    bool
	Hostname string
	MAC      string
	RTT      time.Duration
}

const (
	timeout    = 1500 * time.Millisecond
	concurrent = 32
	// attempts controls how many ping rounds we run per host. A single ICMP
	// probe regularly loses a reply on a fresh ARP cache or a busy network,
	// which made discovery need multiple manual clicks to find everything.
	// Running a few rounds up front means one click finds (almost) all hosts.
	attempts = 3
)

// Subnet pings every host address in the given CIDR and returns the ones that
// responded (plus the total number of addresses that were probed), resolving
// each one's MAC address via raw ARP requests where possible (see arp.go).
// It tries unprivileged ICMP echo first; if that fails to even open a socket
// (no CAP_NET_RAW / no ping group permission), it falls back to a TCP-connect
// probe against a handful of common ports.
func Subnet(ctx context.Context, cidr string) ([]Host, int, error) {
	ips, err := hostsInCIDR(cidr)
	if err != nil {
		return nil, 0, err
	}

	var pinger *icmpPinger
	if icmpAvailable() {
		pinger, _ = newICMPPinger()
	}
	if pinger != nil {
		defer pinger.close()
	}

	resolver := newARPResolver(cidr)
	if resolver != nil {
		defer resolver.close()
	}

	alive := make(map[string]time.Duration)
	var aliveMu sync.Mutex

	for round := 0; round < attempts; round++ {
		if resolver != nil {
			for _, ip := range ips {
				resolver.request(ip)
			}
		}

		sem := make(chan struct{}, concurrent)
		var wg sync.WaitGroup
		for _, ip := range ips {
			aliveMu.Lock()
			_, already := alive[ip]
			aliveMu.Unlock()
			if already {
				continue
			}

			wg.Add(1)
			sem <- struct{}{}
			go func(ip string) {
				defer wg.Done()
				defer func() { <-sem }()

				var ok bool
				var rtt time.Duration
				if pinger != nil {
					ok, rtt = pinger.ping(ip, round+1)
				} else {
					ok, rtt = pingTCP(ctx, ip)
				}
				if !ok {
					return
				}
				aliveMu.Lock()
				alive[ip] = rtt
				aliveMu.Unlock()
			}(ip)
		}
		wg.Wait()

		select {
		case <-ctx.Done():
			round = attempts // stop early if the caller's deadline is up
		default:
		}
	}

	// Give straggling ARP replies a brief moment to arrive after the last
	// ping round before we read them back.
	if resolver != nil {
		time.Sleep(100 * time.Millisecond)
	}

	out := make([]Host, 0, len(alive))
	for ip, rtt := range alive {
		host := Host{IP: ip, Alive: true, RTT: rtt}
		if names, err := net.DefaultResolver.LookupAddr(ctx, ip); err == nil && len(names) > 0 {
			host.Hostname = names[0]
		}
		if resolver != nil {
			host.MAC = resolver.lookup(ip)
		}
		out = append(out, host)
	}
	return out, len(ips), nil
}

func hostsInCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}
	if ip.To4() == nil {
		return nil, fmt.Errorf("only IPv4 subnets are supported")
	}

	var ips []string
	for cur := ip.Mask(ipnet.Mask); ipnet.Contains(cur); inc(cur) {
		ips = append(ips, cur.String())
	}

	// Drop network and broadcast addresses for subnets larger than /31.
	ones, bits := ipnet.Mask.Size()
	if bits-ones >= 2 && len(ips) >= 2 {
		ips = ips[1 : len(ips)-1]
	}

	if len(ips) > 65536 {
		return nil, fmt.Errorf("subnet too large (%d hosts), max is /16", len(ips))
	}

	return ips, nil
}

func inc(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
