package sysinfo

import (
	"testing"
	"time"
)

func TestLiveSampleOnThisMachine(t *testing.T) {
	s := New()
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
