package micropanel

// Internals of the Windows HID layer for the external tests.
var ControlInterface = controlInterface

// BadReportSizeErrors runs every report I/O with a bad size on an unopened
// device and returns the errors, in order: getFeature 0, -1, oversized;
// setFeature empty, oversized; write empty, oversized.
func BadReportSizeErrors() []error {
	d := &hidDevice{outLen: 33, featLen: 64}
	_, e1 := d.getFeature(repAnalog, 0)
	_, e2 := d.getFeature(repAnalog, -1)
	_, e3 := d.getFeature(repAnalog, maxReportLen+1)
	return []error{e1, e2, e3, d.setFeature(nil), d.setFeature(make([]byte, maxReportLen+1)),
		d.write(nil), d.write(make([]byte, maxReportLen+1))}
}
