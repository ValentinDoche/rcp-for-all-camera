package sysinfo_test

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
	"net"
	"testing"
	"time"
)

type fakeReader struct{ raws []sysinfo.Counters }

func (f *fakeReader) Read() (sysinfo.Counters, error) {
	r := f.raws[0]
	if len(f.raws) > 1 {
		f.raws = f.raws[1:]
	}
	return r, nil
}

func TestFirstSampleHasMemoryButNoRates(t *testing.T) {
	s := sysinfo.NewSampler(&fakeReader{raws: []sysinfo.Counters{{MemUsed: 4 << 30, MemTotal: 16 << 30, Rx: 100, Tx: 50, At: time.Unix(0, 0)}}})
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
	s := sysinfo.NewSampler(&fakeReader{raws: []sysinfo.Counters{
		{Idle: 1000, Total: 4000, At: t0},
		{Idle: 1250, Total: 5000, At: t0.Add(time.Second)}, // 1000 ticks, 250 idle
	}})
	_, _ = s.Sample()
	st, _ := s.Sample()
	if !st.HasRates || st.CPU != 75 {
		t.Fatalf("CPU = %v (rates %v), want 75", st.CPU, st.HasRates)
	}
}

func TestNetworkRatesPerSecond(t *testing.T) {
	t0 := time.Unix(0, 0)
	s := sysinfo.NewSampler(&fakeReader{raws: []sysinfo.Counters{
		{Rx: 1000, Tx: 500, At: t0},
		{Rx: 3000, Tx: 1500, At: t0.Add(2 * time.Second)},
	}})
	_, _ = s.Sample()
	st, _ := s.Sample()
	if st.DownBps != 1000 || st.UpBps != 500 {
		t.Fatalf("down %v up %v, want 1000/500 B/s", st.DownBps, st.UpBps)
	}
}

func TestCounterResetGivesZeroNotNegative(t *testing.T) {
	t0 := time.Unix(0, 0)
	s := sysinfo.NewSampler(&fakeReader{raws: []sysinfo.Counters{
		{Rx: 9000, Tx: 9000, Idle: 10, Total: 10, At: t0},
		{Rx: 100, Tx: 100, Idle: 5, Total: 5, At: t0.Add(time.Second)}, // adapter reset
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
	if got := sysinfo.PickIP(addrs); got != "192.168.1.42" {
		t.Fatalf("pickIP = %q, want 192.168.1.42", got)
	}
}

func TestPickIPFallsBackToAnyGlobalIPv4(t *testing.T) {
	addrs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("100.101.1.2")}
	if got := sysinfo.PickIP(addrs); got != "100.101.1.2" {
		t.Fatalf("pickIP = %q", got)
	}
	if got := sysinfo.PickIP([]net.IP{net.ParseIP("127.0.0.1")}); got != "" {
		t.Fatalf("sysinfo.PickIP(loopback only) = %q, want empty", got)
	}
}
