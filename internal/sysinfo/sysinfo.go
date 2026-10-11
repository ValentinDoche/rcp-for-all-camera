// Package sysinfo samples host statistics for the GUI status bar: CPU load,
// memory, network throughput and the machine's LAN address.
package sysinfo

import (
	"errors"
	"net"
	"time"
)

// ErrUnsupported is returned on platforms without an implementation.
var ErrUnsupported = errors.New("sysinfo: unsupported platform")

// Stats is one sample. Rates (CPU, DownBps, UpBps) are averages since the
// previous sample and only valid when HasRates is true.
type Stats struct {
	CPU      float64 // percent, 0..100
	MemUsed  uint64  // bytes
	MemTotal uint64  // bytes
	DownBps  float64 // bytes per second received, all physical interfaces
	UpBps    float64 // bytes per second sent
	IP       string  // main LAN IPv4 address, "" if none
	HasRates bool
}

// raw is what a platform reader returns: cumulative counters.
type raw struct {
	idle, total       uint64 // CPU time ticks
	memUsed, memTotal uint64
	rx, tx            uint64 // cumulative bytes
	ip                string
	at                time.Time
}

type reader interface {
	read() (raw, error)
}

// Sampler turns cumulative counters into rates. Not safe for concurrent use.
type Sampler struct {
	r    reader
	prev *raw
}

// New returns a sampler for this machine.
func New() *Sampler { return newSampler(platformReader{}) }

func newSampler(r reader) *Sampler { return &Sampler{r: r} }

// Sample reads the counters and computes rates since the previous call.
func (s *Sampler) Sample() (Stats, error) {
	cur, err := s.r.read()
	if err != nil {
		return Stats{}, err
	}
	st := Stats{MemUsed: cur.memUsed, MemTotal: cur.memTotal, IP: cur.ip}
	if p := s.prev; p != nil {
		st.HasRates = true
		if dt := cur.total - p.total; cur.total > p.total && cur.idle >= p.idle {
			st.CPU = 100 * float64(dt-(cur.idle-p.idle)) / float64(dt)
		}
		if sec := cur.at.Sub(p.at).Seconds(); sec > 0 {
			st.DownBps = rate(p.rx, cur.rx, sec)
			st.UpBps = rate(p.tx, cur.tx, sec)
		}
	}
	s.prev = &cur
	return st, nil
}

// rate is 0 when a counter went backwards (adapter reset or unplugged).
func rate(prev, cur uint64, sec float64) float64 {
	if cur < prev {
		return 0
	}
	return float64(cur-prev) / sec
}

// pickIP returns the address to show: a private IPv4 first, else any global
// unicast IPv4. Loopback and link-local addresses are skipped.
func pickIP(addrs []net.IP) string {
	var fallback string
	for _, ip := range addrs {
		v4 := ip.To4()
		if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() || !v4.IsGlobalUnicast() {
			continue
		}
		if v4.IsPrivate() {
			return v4.String()
		}
		if fallback == "" {
			fallback = v4.String()
		}
	}
	return fallback
}

// interfaceIPs lists the addresses of interfaces that are up.
func interfaceIPs() []net.IP {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []net.IP
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				out = append(out, n.IP)
			}
		}
	}
	return out
}

// Interface is one local IPv4 address.
type Interface struct {
	Name string
	IP   string
}

// LocalIPv4s lists the IPv4 addresses of the interfaces that are up,
// loopback and link-local excluded, in interface order.
func LocalIPv4s() []Interface {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []Interface
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if v4 := n.IP.To4(); v4 != nil && v4.IsGlobalUnicast() && !v4.IsLinkLocalUnicast() {
				out = append(out, Interface{Name: ifc.Name, IP: v4.String()})
			}
		}
	}
	return out
}
