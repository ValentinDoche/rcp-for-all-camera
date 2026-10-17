package micropanel

import "errors"

// Present reports whether a Micro Camera Panel HID interface is plugged in.
// It only enumerates devices: nothing is opened, so it is cheap to poll and
// does not disturb another program that drives the panel.
func Present() (bool, error) {
	_, err := findHIDPath(VendorID, ProductID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Info is one plugged-in panel.
type Info struct {
	Path   string // HID device path
	Serial string // USB serial number ("" when unreadable)
}

// List returns every plugged-in panel (none: empty, no error). Reading the
// serial opens a shared handle without sending anything, so it does not
// disturb a program that drives the panel.
func List() ([]Info, error) {
	paths, err := findHIDPaths(VendorID, ProductID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(paths))
	for _, p := range paths {
		sn, _ := serialOf(p) // a panel being unplugged just has no serial
		out = append(out, Info{Path: p, Serial: sn})
	}
	return out, nil
}
