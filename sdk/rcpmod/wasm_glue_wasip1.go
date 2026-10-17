//go:build wasip1

package rcpmod

import (
	"encoding/json"
	"errors"
	"runtime"
	"unsafe"
)

// The WebAssembly side of the ABI (docs/modules-camera.html, ABI annex).
// The host never calls back into the module while the module waits in a
// host import: answers to rcp.call are fetched with rcp.take instead.

//go:wasmimport rcp call
func hostCall(ptr unsafe.Pointer, n uint32) uint64

//go:wasmimport rcp take
func hostTake(id uint32, ptr unsafe.Pointer)

//go:wasmimport rcp log
func hostLog(level int32, ptr unsafe.Pointer, n uint32)

var (
	disp   *dispatcher
	inbox  [][]byte // buffers the host wrote requests into, kept until the next call
	result []byte   // last response, valid until the next call
)

//go:wasmexport rcp_alloc
func rcpAlloc(n uint32) unsafe.Pointer {
	b := make([]byte, max(n, 1))
	inbox = append(inbox, b)
	return unsafe.Pointer(unsafe.SliceData(b))
}

//go:wasmexport rcp_call
func rcpCall(ptr unsafe.Pointer, n uint32) uint64 {
	in := unsafe.Slice((*byte)(ptr), n)
	if disp == nil {
		if registered == nil {
			result = encode(Response{Error: &WireError{Code: CodeInternal, Message: "no module registered"}})
			return pack(result)
		}
		disp = newDispatcher(registered, callHost, logHost)
	}
	result = disp.handle(in)
	inbox = inbox[:0]
	return pack(result)
}

func pack(b []byte) uint64 {
	return uint64(uintptr(unsafe.Pointer(unsafe.SliceData(b))))<<32 | uint64(len(b))
}

func callHost(req HostRequest) (HostResponse, error) {
	in, err := json.Marshal(req)
	if err != nil {
		return HostResponse{}, err
	}
	r := hostCall(unsafe.Pointer(unsafe.SliceData(in)), uint32(len(in)))
	runtime.KeepAlive(in)
	id, n := uint32(r>>32), uint32(r)
	out := make([]byte, max(n, 1))
	hostTake(id, unsafe.Pointer(unsafe.SliceData(out)))
	var resp HostResponse
	if err := json.Unmarshal(out[:n], &resp); err != nil {
		return HostResponse{}, err
	}
	if resp.Error != "" {
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

func logHost(level Level, msg string) {
	b := []byte(msg)
	hostLog(int32(level), unsafe.Pointer(unsafe.SliceData(b)), uint32(len(b)))
	runtime.KeepAlive(b)
}
