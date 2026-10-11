//go:build windows

package panel

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// OpenUSB opens and authenticates the ATEM Micro Camera Panel.
func OpenUSB() (Device, error) {
	p, err := micropanel.Open()
	if errors.Is(err, micropanel.ErrNotFound) {
		return nil, ErrNoDevice
	}
	if err != nil {
		return nil, err
	}
	return p, nil
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
