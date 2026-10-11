package micropanel

import "testing"

// Input reports come from a USB device: treat them as untrusted. parseReport
// must never panic, and must only accept reports long enough for their ID.
func FuzzParseReport(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{repKeys, 0x40, 0, 0x41, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add(append([]byte{repAnalog}, make([]byte, 18)...))
	f.Add(append([]byte{repJoystick}, make([]byte, 12)...))
	f.Add([]byte{repKeys, 0x40})
	f.Add([]byte{repJoystick, 1, 2, 3})
	minLen := map[byte]int{repKeys: 13, repAnalog: 19, repJoystick: 13}
	f.Fuzz(func(t *testing.T, p []byte) {
		var s State
		s.Keys = make([]Key, 0, 6)
		ok := s.parseReport(p)
		if len(s.Keys) > 6 {
			t.Fatalf("%d keys decoded, max 6", len(s.Keys))
		}
		if !ok {
			return
		}
		need, known := minLen[p[0]]
		if !known || len(p) < need {
			t.Fatalf("accepted report % x", p)
		}
		for _, k := range s.Keys {
			if k == 0 {
				t.Fatal("zero key code kept")
			}
		}
	})
}
