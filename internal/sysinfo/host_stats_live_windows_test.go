package sysinfo_test

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
	"testing"
	"time"
)

func TestLiveSampleOnThisMachine(t *testing.T) {
	s := sysinfo.New()
	if _, err := s.Sample(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	st, err := s.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if st.MemTotal == 0 || st.MemUsed > st.MemTotal || st.CPU < 0 || st.CPU > 100 {
		t.Fatalf("implausible sample: %+v", st)
	}
	t.Logf("%+v", st)
}

func TestLocalIPv4sOnThisMachine(t *testing.T) {
	for _, n := range sysinfo.LocalIPv4s() {
		if n.IP == "" || n.Name == "" {
			t.Fatalf("bad interface %+v", n)
		}
		t.Logf("%s %s", n.Name, n.IP)
	}
}
