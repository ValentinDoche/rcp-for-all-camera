package sysinfo

import (
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

// memoryStatusEx is MEMORYSTATUSEX.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type platformReader struct{}

func (platformReader) Read() (Counters, error) {
	r := Counters{At: time.Now()}

	var idle, kernel, user windows.Filetime
	if ok, _, err := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); ok == 0 {
		return Counters{}, fmt.Errorf("GetSystemTimes: %w", err)
	}
	// Kernel time already includes idle time.
	r.Idle = ft(idle)
	r.Total = ft(kernel) + ft(user)

	m := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if ok, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); ok == 0 {
		return Counters{}, fmt.Errorf("GlobalMemoryStatusEx: %w", err)
	}
	r.MemTotal = m.TotalPhys
	r.MemUsed = m.TotalPhys - m.AvailPhys

	r.Rx, r.Tx = netCounters()
	r.IP = PickIP(interfaceIPs())
	return r, nil
}

func ft(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }

// netCounters sums the byte counters of the interfaces that are up, except
// loopback. Per-interface failures are skipped: the bar is informative only.
func netCounters() (rx, tx uint64) {
	ifs, err := net.Interfaces()
	if err != nil {
		return 0, 0
	}
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		row := windows.MibIfRow2{InterfaceIndex: uint32(ifc.Index)}
		if windows.GetIfEntry2Ex(windows.MibIfEntryNormal, &row) != nil {
			continue
		}
		rx += row.InOctets
		tx += row.OutOctets
	}
	return rx, tx
}
