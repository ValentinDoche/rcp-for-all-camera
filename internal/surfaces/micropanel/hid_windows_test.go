//go:build windows

package micropanel_test

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
	"testing"
)

// Report buffers are indexed at [0] before the syscall: bad sizes must be
// rejected, not panic.
func TestHIDRejectsBadReportSizes(t *testing.T) {
	for i, err := range micropanel.BadReportSizeErrors() {
		if err == nil {
			t.Errorf("bad report size case %d: nil error", i)
		}
	}
}

func TestControlInterface(t *testing.T) {
	const needle = "vid_1edb&pid_bf01"
	for path, want := range map[string]bool{
		`\?\hid#vid_1edb&pid_bf01&mi_02#7&1&0000#{4d1e55b2}`: true,
		`\?\hid#vid_1edb&pid_bf01&mi_00#7&2&0000#{4d1e55b2}`: false,
		`\?\hid#vid_1edb&pid_bf01#7&3&0000#{4d1e55b2}`:       true,
		`\?\hid#vid_046d&pid_c52b&mi_02#7&4&0000#{4d1e55b2}`: false,
	} {
		if got := micropanel.ControlInterface(path, needle); got != want {
			t.Errorf("controlInterface(%s) = %v, want %v", path, got, want)
		}
	}
}
