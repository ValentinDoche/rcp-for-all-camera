//go:build !windows

package panel

// OpenUSB: the USB panel driver is Windows only for now.
func OpenUSB() (Device, error) { return nil, ErrNoDevice }

// OtherOwnerRunning: no ATEM Software Control check outside Windows.
func OtherOwnerRunning() bool { return false }
