// Command winres builds the Windows icon resource of rcp.exe from the app
// logo (internal/gui/brand/logo.svg). Run it from cmd/rcp through
//
//	go generate ./cmd/rcp
//
// It writes winres/rcp.ico, winres/rcp.rc and rsrc_windows_amd64.syso, which
// the Go linker adds to rcp.exe on its own. It needs windres (MinGW-w64).
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const (
	logo = "../../internal/gui/brand/logo.svg"
	ico  = "winres/rcp.ico"
	rc   = "winres/rcp.rc"
	syso = "rsrc_windows_amd64.syso"
)

// sizes are the icon frames Windows picks from (Explorer, taskbar, Alt-Tab).
var sizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "winres:", err)
		os.Exit(1)
	}
}

func run() error {
	var frames [][]byte
	for _, s := range sizes {
		b, err := render(s)
		if err != nil {
			return err
		}
		frames = append(frames, b)
	}
	if err := os.WriteFile(ico, icoFile(frames), 0o644); err != nil {
		return err
	}
	// The resource name sorts first, so Explorer shows this icon for the exe.
	if err := os.WriteFile(rc, []byte("1 ICON \"rcp.ico\"\n"), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("windres", "-O", "coff", "-F", "pe-x86-64", "-i", rc, "-o", syso)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// render rasterises the logo into a size×size PNG.
func render(size int) ([]byte, error) {
	f, err := os.Open(logo)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	icon, err := oksvg.ReadIconStream(f)
	if err != nil {
		return nil, err
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, img, img.Bounds())), 1)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// icoFile packs PNG frames into an ICO file (Vista+ accepts PNG frames).
func icoFile(frames [][]byte) []byte {
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	le([3]uint16{0, 1, uint16(len(frames))}) // reserved, type icon, count
	offset := 6 + 16*len(frames)
	for i, fr := range frames {
		dim := uint8(sizes[i] % 256) // 0 means 256
		le([4]uint8{dim, dim, 0, 0}) // width, height, colours, reserved
		le([2]uint16{1, 32})         // planes, bits per pixel
		le([2]uint32{uint32(len(fr)), uint32(offset)})
		offset += len(fr)
	}
	for _, fr := range frames {
		b.Write(fr)
	}
	return b.Bytes()
}
