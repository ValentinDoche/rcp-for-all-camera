//go:build !windows

package panel

// OpenUSB: the USB panel driver is Windows only for now.
func OpenUSB() (Device, error) { return nil, ErrNoDevice }

// OpenUSBSerial: Windows only for now.
func OpenUSBSerial(string) (Device, error) { return nil, ErrNoDevice }

// USBPanels: Windows only for now.
func USBPanels() ([]string, error) { return nil, nil }

// OtherOwnerRunning: no ATEM Software Control check outside Windows.
func OtherOwnerRunning() bool { return false }
