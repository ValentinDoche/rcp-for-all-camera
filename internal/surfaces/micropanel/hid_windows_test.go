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
