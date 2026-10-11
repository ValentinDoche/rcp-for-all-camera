//go:build windows

package config

// Windows DPAPI (CryptProtectData / CryptUnprotectData) through crypt32.dll,
// without cgo or golang.org/x/sys, like internal/surfaces/micropanel.

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")
)

// cryptprotectUIForbidden makes DPAPI fail instead of showing a prompt.
const cryptprotectUIForbidden = 0x1

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func blobOf(b []byte) *dataBlob {
	if len(b) == 0 {
		return nil
	}
	return &dataBlob{cbData: uint32(len(b)), pbData: &b[0]}
}

// dpapiAvailable reports whether the DLL entry points load (LazyProc.Call
// panics otherwise).
func dpapiAvailable() error {
	for _, p := range []*syscall.LazyProc{procCryptProtectData, procCryptUnprotectData, procLocalFree} {
		if err := p.Find(); err != nil {
			return err
		}
	}
	return nil
}

// dpapiProtect encrypts plain for the current Windows user (user scope: no
// CRYPTPROTECT_LOCAL_MACHINE), bound to entropy.
func dpapiProtect(plain, entropy []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("CryptProtectData: empty input")
	}
	var out dataBlob
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(blobOf(plain))), 0,
		uintptr(unsafe.Pointer(blobOf(entropy))), 0, 0,
		cryptprotectUIForbidden, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(plain)
	runtime.KeepAlive(entropy)
	if r == 0 {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return append([]byte(nil), unsafe.Slice(out.pbData, out.cbData)...), nil
}

// dpapiUnprotect decrypts a blob made by dpapiProtect with the same entropy.
// The OS-allocated plaintext is wiped before it is freed; the caller clears
// the returned slice once done.
func dpapiUnprotect(blob, entropy []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, errors.New("CryptUnprotectData: empty input")
	}
	var out dataBlob
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(blobOf(blob))), 0,
		uintptr(unsafe.Pointer(blobOf(entropy))), 0, 0,
		cryptprotectUIForbidden, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(blob)
	runtime.KeepAlive(entropy)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	sys := unsafe.Slice(out.pbData, out.cbData)
	plain := append([]byte(nil), sys...)
	clear(sys)
	return plain, nil
}
