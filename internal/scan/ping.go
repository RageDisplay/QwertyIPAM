package scan

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func icmpAvailable() bool {
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// icmpPinger sends echo requests and collects replies over a single shared
// socket instead of opening one socket per host. Opening dozens of unprivileged
// ICMP sockets concurrently (one per host, as a naive implementation would)
// puts a lot of churn on the NAT/conntrack layer between the container and
// the target network (e.g. Docker Desktop's WSL2/Hyper-V VM), which was
// silently dropping replies for some hosts under load. A single long-lived
// socket avoids that entirely.
type icmpPinger struct {
	conn *icmp.PacketConn
	id   int

	mu      sync.Mutex
	pending map[string]chan time.Time // "ip:seq" -> reply channel
}

func newICMPPinger() (*icmpPinger, error) {
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return nil, err
	}
	p := &icmpPinger{
		conn:    conn,
		id:      os.Getpid() & 0xffff,
		pending: make(map[string]chan time.Time),
	}
	go p.readLoop()
	return p, nil
}

func (p *icmpPinger) readLoop() {
	rb := make([]byte, 1500)
	for {
		n, peer, err := p.conn.ReadFrom(rb)
		if err != nil {
			return // socket closed
		}
		rm, err := icmp.ParseMessage(1, rb[:n])
		if err != nil || rm.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		echo, ok := rm.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		ip := peerIP(peer)
		key := fmt.Sprintf("%s:%d", ip, echo.Seq)
		p.mu.Lock()
		ch, found := p.pending[key]
		p.mu.Unlock()
		if found {
			select {
			case ch <- time.Now():
			default:
			}
		}
	}
}

func peerIP(addr net.Addr) string {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP.String()
	case *net.IPAddr:
		return a.IP.String()
	default:
		return addr.String()
	}
}

// ping sends a single echo request to ip and waits up to `timeout` for a
// reply. seq should be unique per (ip, round) pair within the lifetime of
// this pinger so replies don't get matched to the wrong request.
func (p *icmpPinger) ping(ip string, seq int) (bool, time.Duration) {
	dst, err := net.ResolveIPAddr("ip4", ip)
	if err != nil {
		return false, 0
	}

	key := fmt.Sprintf("%s:%d", ip, seq)
	ch := make(chan time.Time, 1)
	p.mu.Lock()
	p.pending[key] = ch
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, key)
		p.mu.Unlock()
	}()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   p.id,
			Seq:  seq,
			Data: []byte("vectrixipam"),
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return false, 0
	}

	start := time.Now()
	if _, err := p.conn.WriteTo(wb, &net.UDPAddr{IP: dst.IP}); err != nil {
		return false, 0
	}

	select {
	case t := <-ch:
		return true, t.Sub(start)
	case <-time.After(timeout):
		return false, 0
	}
}

func (p *icmpPinger) close() {
	_ = p.conn.Close()
}

var tcpProbePorts = []string{"80", "443", "22", "3389", "445"}

func pingTCP(ctx context.Context, ip string) (bool, time.Duration) {
	start := time.Now()
	for _, port := range tcpProbePorts {
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, port))
		if err == nil {
			conn.Close()
			return true, time.Since(start)
		}
	}
	return false, 0
}
