package sysinfo

import (
	"net"
	"testing"
	"time"
)

type fakeReader struct{ raws []raw }

func (f *fakeReader) read() (raw, error) {
	r := f.raws[0]
	if len(f.raws) > 1 {
		f.raws = f.raws[1:]
	}
	return r, nil
}

func TestFirstSampleHasMemoryButNoRates(t *testing.T) {
	s := newSampler(&fakeReader{raws: []raw{{memUsed: 4 << 30, memTotal: 16 << 30, rx: 100, tx: 50, at: time.Unix(0, 0)}}})
	st, err := s.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if st.MemUsed != 4<<30 || st.MemTotal != 16<<30 {
		t.Fatalf("memory = %d/%d", st.MemUsed, st.MemTotal)
	}
	if st.HasRates {
		t.Fatal("first sample must not report rates")
	}
}

func TestCPUPercentFromSystemTimes(t *testing.T) {
	t0 := time.Unix(0, 0)
	s := newSampler(&fakeReader{raws: []raw{
		{idle: 1000, total: 4000, at: t0},
		{idle: 1250, total: 5000, at: t0.Add(time.Second)}, // 1000 ticks, 250 idle
	}})
	_, _ = s.Sample()
	st, _ := s.Sample()
	if !st.HasRates || st.CPU != 75 {
		t.Fatalf("CPU = %v (rates %v), want 75", st.CPU, st.HasRates)
	}
}

func TestNetworkRatesPerSecond(t *testing.T) {
	t0 := time.Unix(0, 0)
	s := newSampler(&fakeReader{raws: []raw{
		{rx: 1000, tx: 500, at: t0},
		{rx: 3000, tx: 1500, at: t0.Add(2 * time.Second)},
	}})
	_, _ = s.Sample()
	st, _ := s.Sample()
	if st.DownBps != 1000 || st.UpBps != 500 {
		t.Fatalf("down %v up %v, want 1000/500 B/s", st.DownBps, st.UpBps)
	}
}

func TestCounterResetGivesZeroNotNegative(t *testing.T) {
	t0 := time.Unix(0, 0)
	s := newSampler(&fakeReader{raws: []raw{
		{rx: 9000, tx: 9000, idle: 10, total: 10, at: t0},
		{rx: 100, tx: 100, idle: 5, total: 5, at: t0.Add(time.Second)}, // adapter reset
	}})
	_, _ = s.Sample()
	st, _ := s.Sample()
	if st.DownBps != 0 || st.UpBps != 0 || st.CPU != 0 {
		t.Fatalf("after reset: %+v, want zero rates", st)
	}
}

func TestPickIPPrefersPrivateIPv4OverOthers(t *testing.T) {
	addrs := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("fe80::1"),
		net.ParseIP("169.254.10.2"),
		net.ParseIP("100.101.1.2"),
		net.ParseIP("192.168.1.42"),
	}
	if got := pickIP(addrs); got != "192.168.1.42" {
		t.Fatalf("pickIP = %q, want 192.168.1.42", got)
	}
}

func TestPickIPFallsBackToAnyGlobalIPv4(t *testing.T) {
	addrs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("100.101.1.2")}
	if got := pickIP(addrs); got != "100.101.1.2" {
		t.Fatalf("pickIP = %q", got)
	}
	if got := pickIP([]net.IP{net.ParseIP("127.0.0.1")}); got != "" {
		t.Fatalf("pickIP(loopback only) = %q, want empty", got)
	}
}
