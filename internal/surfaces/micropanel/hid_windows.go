//go:build windows

package micropanel

// Minimal pure-Go (no cgo) access to a HID device on Windows through
// setupapi.dll, hid.dll and kernel32. Two handles are opened on the same
// device path: one dedicated to blocking input reads, one for output and
// feature reports, because synchronous handles serialize their I/O.

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	setupapi = syscall.NewLazyDLL("setupapi.dll")
	hiddll   = syscall.NewLazyDLL("hid.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetupDiGetClassDevsW             = setupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces      = setupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetailW = setupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDeviceInfoList     = setupapi.NewProc("SetupDiDestroyDeviceInfoList")

	procHidDGetHidGuid         = hiddll.NewProc("HidD_GetHidGuid")
	procHidDGetAttributes      = hiddll.NewProc("HidD_GetAttributes")
	procHidDSetFeature         = hiddll.NewProc("HidD_SetFeature")
	procHidDGetFeature         = hiddll.NewProc("HidD_GetFeature")
	procHidDGetPreparsedData   = hiddll.NewProc("HidD_GetPreparsedData")
	procHidDFreePreparsedData  = hiddll.NewProc("HidD_FreePreparsedData")
	procHidPGetCaps            = hiddll.NewProc("HidP_GetCaps")
	procHidDGetSerialNumberStr = hiddll.NewProc("HidD_GetSerialNumberString")

	procCancelIoEx = kernel32.NewProc("CancelIoEx")
)

const (
	digcfPresent         = 0x02
	digcfDeviceInterface = 0x10
	invalidHandle        = ^uintptr(0)
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type spDeviceInterfaceData struct {
	CbSize   uint32
	Class    guid
	Flags    uint32
	Reserved uintptr
}

type hiddAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
}

type hidpCaps struct {
	Usage                     uint16
	UsagePage                 uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

// maxReportLen bounds every report buffer. The panel's reports are at most
// 64 bytes (full-speed HID); larger values come from a bug or a bad device.
const maxReportLen = 64

// ErrNotFound is returned when no matching HID interface is present.
var ErrNotFound = errors.New("micropanel: device not found")

// hidDevice is an open HID interface.
type hidDevice struct {
	path     string
	rd, wr   syscall.Handle
	inLen    int
	outLen   int
	featLen  int
	serial   string
	closeMu  sync.Mutex
	isClosed bool
}

// findHIDPath returns the device path of the first HID interface matching vid/pid.
func findHIDPath(vid, pid uint16) (string, error) {
	paths, err := findHIDPaths(vid, pid)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// findHIDPaths returns the device paths of every HID interface matching
// vid/pid (ErrNotFound when there is none). On a composite device only
// interface MI_02 is kept: the panel's MI_00 is not the control interface.
func findHIDPaths(vid, pid uint16) ([]string, error) {
	var g guid
	procHidDGetHidGuid.Call(uintptr(unsafe.Pointer(&g)))
	set, _, err := procSetupDiGetClassDevsW.Call(uintptr(unsafe.Pointer(&g)), 0, 0, digcfPresent|digcfDeviceInterface)
	if set == invalidHandle {
		return nil, fmt.Errorf("SetupDiGetClassDevs: %w", err)
	}
	defer procSetupDiDestroyDeviceInfoList.Call(set)

	needle := strings.ToLower(fmt.Sprintf("vid_%04x&pid_%04x", vid, pid))
	var paths []string
	for i := uint32(0); ; i++ {
		var ifd spDeviceInterfaceData
		ifd.CbSize = uint32(unsafe.Sizeof(ifd))
		ok, _, _ := procSetupDiEnumDeviceInterfaces.Call(set, 0, uintptr(unsafe.Pointer(&g)), uintptr(i), uintptr(unsafe.Pointer(&ifd)))
		if ok == 0 {
			if len(paths) == 0 {
				return nil, ErrNotFound
			}
			return paths, nil
		}
		var need uint32
		procSetupDiGetDeviceInterfaceDetailW.Call(set, uintptr(unsafe.Pointer(&ifd)), 0, 0, uintptr(unsafe.Pointer(&need)), 0)
		// cbSize (4 bytes) plus at least one UTF-16 char, and a sane upper
		// bound: the cbSize write below must stay inside buf.
		if need < 8 || need > 64<<10 {
			continue
		}
		buf := make([]byte, need)
		// SP_DEVICE_INTERFACE_DETAIL_DATA_W.cbSize: 8 on 64-bit, 6 on 32-bit.
		cb := uint32(8)
		if unsafe.Sizeof(uintptr(0)) == 4 {
			cb = 6
		}
		*(*uint32)(unsafe.Pointer(&buf[0])) = cb
		ok, _, _ = procSetupDiGetDeviceInterfaceDetailW.Call(set, uintptr(unsafe.Pointer(&ifd)), uintptr(unsafe.Pointer(&buf[0])), uintptr(need), 0, 0)
		if ok == 0 {
			continue
		}
		u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[4])), (len(buf)-4)/2)
		path := syscall.UTF16ToString(u16)
		if controlInterface(path, needle) {
			paths = append(paths, path)
		}
	}
}

// controlInterface reports whether a device path is the panel's HID
// control interface: it matches vid/pid and is MI_02 (or not composite).
func controlInterface(path, needle string) bool {
	l := strings.ToLower(path)
	if !strings.Contains(l, needle) {
		return false
	}
	return !strings.Contains(l, "&mi_") || strings.Contains(l, "&mi_02")
}

func openHandle(path string) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
}

// serialOf reads the USB serial number of the device at path without
// claiming it (shared handle, no report sent).
func serialOf(path string) (string, error) {
	h, err := openHandle(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer syscall.CloseHandle(h)
	return readSerial(h), nil
}

func readSerial(h syscall.Handle) string {
	var sn [128]uint16
	if ok, _, _ := procHidDGetSerialNumberStr.Call(uintptr(h), uintptr(unsafe.Pointer(&sn[0])), uintptr(len(sn)*2)); ok != 0 {
		return syscall.UTF16ToString(sn[:])
	}
	return ""
}

func openHIDPath(path string) (*hidDevice, error) {
	rd, err := openHandle(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	wr, err := openHandle(path)
	if err != nil {
		syscall.CloseHandle(rd)
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	d := &hidDevice{path: path, rd: rd, wr: wr}

	var attrs hiddAttributes
	attrs.Size = uint32(unsafe.Sizeof(attrs))
	procHidDGetAttributes.Call(uintptr(wr), uintptr(unsafe.Pointer(&attrs)))

	var pp uintptr
	if ok, _, _ := procHidDGetPreparsedData.Call(uintptr(wr), uintptr(unsafe.Pointer(&pp))); ok != 0 {
		var caps hidpCaps
		procHidPGetCaps.Call(pp, uintptr(unsafe.Pointer(&caps)))
		procHidDFreePreparsedData.Call(pp)
		d.inLen, d.outLen, d.featLen = int(caps.InputReportByteLength), int(caps.OutputReportByteLength), int(caps.FeatureReportByteLength)
	}
	if d.inLen <= 0 || d.inLen > maxReportLen {
		d.inLen, d.outLen, d.featLen = maxReportLen, maxReportLen, maxReportLen
	}

	d.serial = readSerial(wr)
	return d, nil
}

// read blocks until one input report arrives; the first byte is the report ID.
func (d *hidDevice) read() ([]byte, error) {
	buf := make([]byte, d.inLen)
	var n uint32
	if err := syscall.ReadFile(d.rd, buf, &n, nil); err != nil {
		return nil, err
	}
	return buf[:min(int(n), len(buf))], nil
}

// write sends an output report (p[0] = report ID), zero-padded to the device length.
func (d *hidDevice) write(p []byte) error {
	if len(p) == 0 || len(p) > maxReportLen {
		return fmt.Errorf("output report: bad length %d", len(p))
	}
	buf := make([]byte, max(d.outLen, len(p)))
	copy(buf, p)
	var n uint32
	return syscall.WriteFile(d.wr, buf, &n, nil)
}

// setFeature sends a feature report of exactly len(p) bytes: the panel stalls
// (ERROR_GEN_FAILURE) when the buffer is padded to the device maximum.
func (d *hidDevice) setFeature(p []byte) error {
	if len(p) == 0 || len(p) > maxReportLen {
		return fmt.Errorf("feature report: bad length %d", len(p))
	}
	buf := append([]byte(nil), p...)
	ok, _, err := procHidDSetFeature.Call(uintptr(d.wr), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ok == 0 {
		return fmt.Errorf("HidD_SetFeature(0x%02x): %w", p[0], err)
	}
	return nil
}

// getFeature reads feature report id with an exact total size (report ID included).
func (d *hidDevice) getFeature(id byte, size int) ([]byte, error) {
	if size < 1 || size > maxReportLen {
		return nil, fmt.Errorf("feature report 0x%02x: bad size %d", id, size)
	}
	buf := make([]byte, size)
	buf[0] = id
	ok, _, err := procHidDGetFeature.Call(uintptr(d.wr), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ok == 0 {
		return nil, fmt.Errorf("HidD_GetFeature(0x%02x): %w", id, err)
	}
	return buf[:size], nil
}

func (d *hidDevice) close() error {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()
	if d.isClosed {
		return nil
	}
	d.isClosed = true
	procCancelIoEx.Call(uintptr(d.rd), 0)
	syscall.CloseHandle(d.rd)
	return syscall.CloseHandle(d.wr)
}
