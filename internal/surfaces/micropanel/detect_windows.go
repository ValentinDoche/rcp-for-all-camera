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
