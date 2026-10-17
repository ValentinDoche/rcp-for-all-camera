//go:build windows

package panel

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// OpenUSB opens and authenticates the first ATEM Micro Camera Panel.
func OpenUSB() (Device, error) { return OpenUSBSerial("") }

// OpenUSBSerial opens and authenticates the panel with this USB serial
// number ("" = the first one found).
func OpenUSBSerial(serial string) (Device, error) {
	p, err := micropanel.OpenSerial(serial)
	if errors.Is(err, micropanel.ErrNotFound) {
		return nil, ErrNoDevice
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// USBPanels returns the USB serial numbers of the plugged-in panels.
func USBPanels() ([]string, error) {
	list, err := micropanel.List()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, in := range list {
		out = append(out, in.Serial)
	}
	return out, nil
}

// otherOwners are programs that authenticate the panel and overwrite its
// displays (AGENTS.md trap 4).
var otherOwners = []string{"atem software control.exe"}

// OtherOwnerRunning reports whether ATEM Software Control is running. Both
// programs can open the HID device at the same time, so the process list is
// the only reliable sign that the panel is taken.
func OtherOwnerRunning() bool {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(snap)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		name := strings.ToLower(syscall.UTF16ToString(e.ExeFile[:]))
		for _, o := range otherOwners {
			if name == o {
				return true
			}
		}
	}
	return false
}
