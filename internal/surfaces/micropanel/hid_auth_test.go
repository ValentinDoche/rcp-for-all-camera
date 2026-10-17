package micropanel

import "testing"

// Challenges captured from a real ATEM Micro Camera Panel; the panel accepted
// these responses (feature report 0x05 answered 04 = authenticated).
func TestAuthResponse(t *testing.T) {
	cases := []struct{ challenge, want uint64 }{
		{0x1e59a7120fe79e4f, 0x2aa4c73c16735880},
		{0x5787f0df1dd6ed5e, 0xd17270f3309f1c50},
		{0x376860472588f1bc, 0xcea613baf234d976},
	}
	for _, c := range cases {
		if got := authResponse(c.challenge); got != c.want {
			t.Errorf("authResponse(%#x) = %#x, want %#x", c.challenge, got, c.want)
		}
	}
}
