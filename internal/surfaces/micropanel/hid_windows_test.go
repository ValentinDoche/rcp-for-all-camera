//go:build windows

package micropanel

import "testing"

// Report buffers are indexed at [0] before the syscall: bad sizes must be
// rejected, not panic.
func TestHIDRejectsBadReportSizes(t *testing.T) {
	d := &hidDevice{outLen: 33, featLen: 64}
	if _, err := d.getFeature(repAnalog, 0); err == nil {
		t.Error("getFeature(size 0) = nil error")
	}
	if _, err := d.getFeature(repAnalog, -1); err == nil {
		t.Error("getFeature(size -1) = nil error")
	}
	if _, err := d.getFeature(repAnalog, maxReportLen+1); err == nil {
		t.Error("getFeature(oversized) = nil error")
	}
	if err := d.setFeature(nil); err == nil {
		t.Error("setFeature(empty) = nil error")
	}
	if err := d.setFeature(make([]byte, maxReportLen+1)); err == nil {
		t.Error("setFeature(oversized) = nil error")
	}
	if err := d.write(nil); err == nil {
		t.Error("write(empty) = nil error")
	}
	if err := d.write(make([]byte, maxReportLen+1)); err == nil {
		t.Error("write(oversized) = nil error")
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
		if got := controlInterface(path, needle); got != want {
			t.Errorf("controlInterface(%s) = %v, want %v", path, got, want)
		}
	}
}
