//go:build windows

package micropanel

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// reauthEvery keeps the session alive; the panel grants 600 s per handshake.
const reauthEvery = 400 * time.Second

// Panel is an open, authenticated ATEM Micro Camera Panel.
type Panel struct {
	dev      *hidDevice
	mu       sync.Mutex // serializes writes and feature reports
	Firmware string
	authTTL  time.Duration
}

// Open finds the first panel on USB and authenticates it.
func Open() (*Panel, error) { return OpenSerial("") }

// OpenSerial opens and authenticates the panel with this USB serial number
// ("" = the first panel found). ErrNotFound when it is not plugged in.
func OpenSerial(serial string) (*Panel, error) {
	path, err := findHIDPath(VendorID, ProductID)
	if err != nil {
		return nil, err
	}
	if serial != "" {
		list, err := List()
		if err != nil {
			return nil, err
		}
		path = ""
		for _, in := range list {
			if in.Serial == serial {
				path = in.Path
				break
			}
		}
		if path == "" {
			return nil, ErrNotFound
		}
	}
	dev, err := openHIDPath(path)
	if err != nil {
		return nil, err
	}
	p := &Panel{dev: dev}
	if v, err := dev.getFeature(repVersion, 8); err == nil {
		p.Firmware = hex.EncodeToString(v[1:])
	}
	if err := p.authenticate(); err != nil {
		dev.close()
		return nil, err
	}
	// Start from a known state: LED layers survive host restarts.
	if err := p.ClearLEDs(-1); err != nil {
		dev.close()
		return nil, fmt.Errorf("clear LEDs: %w", err)
	}
	return p, nil
}

// Serial returns the USB serial number string.
func (p *Panel) Serial() string { return p.dev.serial }

// AuthTTL returns the session lifetime granted by the last handshake.
func (p *Panel) AuthTTL() time.Duration { return p.authTTL }

func (p *Panel) authenticate() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	step := func(sub byte, payload uint64) ([]byte, error) {
		req := make([]byte, 10)
		req[0], req[1] = repAuth, sub
		binary.LittleEndian.PutUint64(req[2:], payload)
		if err := p.dev.setFeature(req); err != nil {
			return nil, err
		}
		return p.dev.getFeature(repAuth, 10)
	}
	r, err := step(0, 0)
	if err != nil {
		return fmt.Errorf("auth: request challenge: %w", err)
	}
	challenge := binary.LittleEndian.Uint64(r[2:])
	if _, err := step(1, 0); err != nil {
		return fmt.Errorf("auth: step 1: %w", err)
	}
	r, err = step(3, authResponse(challenge))
	if err != nil {
		return fmt.Errorf("auth: send response: %w", err)
	}
	if r[1] != 4 {
		return fmt.Errorf("auth: rejected (state %#x)", r[1])
	}
	p.authTTL = time.Duration(binary.LittleEndian.Uint16(r[2:])) * time.Second
	return nil
}

// Run reads input reports until ctx is cancelled or the device fails, calling
// fn with every decoded state change and raw for every report (raw may be nil).
// It re-authenticates periodically.
func (p *Panel) Run(ctx context.Context, fn func(s State), raw func(report []byte)) error {
	// Knob and lever positions are only reported on change; read the current
	// values from their feature reports so the first state is complete.
	var s State
	for _, f := range []struct {
		id   byte
		size int
	}{{repAnalog, 19}, {repJoystick, 13}} {
		if r, err := p.GetFeature(f.id, f.size); err == nil {
			s.parseReport(r)
		}
	}
	fn(s)

	errc := make(chan error, 1)
	go func() {
		for {
			r, err := p.dev.read()
			if err != nil {
				errc <- err
				return
			}
			if raw != nil {
				raw(r)
			}
			if s.parseReport(r) {
				snap := s
				snap.Keys = append([]Key(nil), s.Keys...)
				fn(snap)
			}
		}
	}()
	t := time.NewTicker(reauthEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			p.dev.close()
			return ctx.Err()
		case err := <-errc:
			p.dev.close()
			return fmt.Errorf("micropanel: read: %w", err)
		case <-t.C:
			if err := p.authenticate(); err != nil {
				p.dev.close()
				return err
			}
		}
	}
}

// Close releases the device.
func (p *Panel) Close() error { return p.dev.close() }

// WriteReport sends a raw output report (b[0] = report ID).
func (p *Panel) WriteReport(b []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dev.write(b)
}

// SetFeature sends a raw feature report (b[0] = report ID).
func (p *Panel) SetFeature(b []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dev.setFeature(b)
}

// GetFeature reads a raw feature report of the given total size.
func (p *Panel) GetFeature(id byte, size int) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dev.getFeature(id, size)
}

// LEDLayers is the number of LED layers in report 0x02.
const LEDLayers = 6

// ClearLEDs turns off every LED on all layers except keep (pass -1 to clear
// all). The panel keeps each layer's bitmap until it is rewritten, so a bit
// left set on a layer the host no longer drives stays lit.
func (p *Panel) ClearLEDs(keep int) error {
	var off LEDBits
	for layer := 0; layer < LEDLayers; layer++ {
		if layer == keep {
			continue
		}
		if err := p.WriteReport(off.Report(byte(layer))); err != nil {
			return err
		}
	}
	return nil
}

// SetDigitsRaw writes the 14 seven-segment digits as raw segment bytes.
func (p *Panel) SetDigitsRaw(seg [14]byte) error {
	return p.WriteReport(append([]byte{repDigits}, seg[:]...))
}

// SetMatrixRaw writes the 32-byte camera dot-matrix buffer (128 pixels x 2 bits).
func (p *Panel) SetMatrixRaw(px [32]byte) error {
	return p.WriteReport(append([]byte{repMatrix}, px[:]...))
}
