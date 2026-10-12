package panel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Run drives the panel until ctx ends (app.Surface). It opens the panel when
// it is present and free, keeps it in sync with the app, survives unplug
// and replug, and blanks the panel before returning.
func (c *Controller) Run(ctx context.Context, api app.SurfaceAPI, report func(app.SurfaceStatus)) {
	if report == nil {
		report = func(app.SurfaceStatus) {}
	}
	w := newWriter(api, c.log, c.opt.Throttle, c.onWriteError)
	w.onScene = c.refresh // a recalled preset changed the camera
	armed := api.StoreArmed()
	c.mu.Lock()
	c.api, c.w, c.storeArmed = api, w, armed
	c.mu.Unlock()
	c.setRig(api.Rig())

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.run(ctx, c.wake, c.refresh)
	}()
	defer func() { <-done }()

	var last app.SurfaceStatus
	set := func(s app.SurfaceStatus) {
		if s != last {
			last = s
			report(s)
		}
	}
	defer set(app.SurfaceStatus{})

	retry := c.opt.RetryMin
	failing := false
	for {
		wait := c.opt.RetryMin
		switch dev, err := c.open(); {
		case errors.Is(err, errBusy):
			set(app.SurfaceStatus{Detail: DetailBusy})
		case errors.Is(err, ErrNoDevice):
			set(app.SurfaceStatus{})
			failing, retry = false, c.opt.RetryMin
		case err != nil:
			if !failing {
				c.log.Warn("panel: open failed, retrying", "err", err)
			}
			failing = true
			set(app.SurfaceStatus{Detail: DetailOpenFail})
			wait, retry = retry, min(2*retry, c.opt.RetryMax)
		default:
			failing, retry = false, c.opt.RetryMin
			c.log.Info("panel: driving", "serial", dev.Serial())
			set(app.SurfaceStatus{Active: true, Serial: dev.Serial()})
			err := c.drive(ctx, dev)
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, errBusy) {
				set(app.SurfaceStatus{Detail: DetailBusy})
			} else {
				c.log.Info("panel: lost", "err", err)
				set(app.SurfaceStatus{})
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

var errBusy = errors.New("panel: held by another program")

func (c *Controller) open() (Device, error) {
	if c.opt.Busy() {
		return nil, errBusy
	}
	return c.opt.Open()
}

// drive runs one panel session. It returns when ctx ends (after blanking
// the panel), the device fails, or another program takes the panel.
func (c *Controller) drive(ctx context.Context, dev Device) error {
	c.mu.Lock()
	c.havePrev = false
	c.releaseLocked()
	c.mu.Unlock()
	c.requestRefresh()

	runCtx, stop := context.WithCancel(context.Background())
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- dev.Run(runCtx, c.onInput, nil) }()
	// end stops the reader, which closes the device.
	end := func(err error) error {
		stop()
		<-errc
		dev.Close()
		return err
	}

	tick := time.NewTicker(c.opt.FrameEvery)
	defer tick.Stop()
	nextBusy := time.Now().Add(c.opt.BusyEvery)
	var last *frame
	for {
		select {
		case <-ctx.Done():
			blank(dev)
			return end(ctx.Err())
		case err := <-errc:
			dev.Close()
			return fmt.Errorf("panel read: %w", err)
		case now := <-tick.C:
			if now.After(nextBusy) {
				nextBusy = now.Add(c.opt.BusyEvery)
				if c.opt.Busy() {
					// The other program now owns the displays: leave them.
					return end(errBusy)
				}
			}
			f := c.frame(now)
			if err := write(dev, f, last); err != nil {
				return end(fmt.Errorf("panel write: %w", err))
			}
			last = &f
			c.recenterKnobs(dev)
		}
	}
}

// write sends the parts of f that differ from last (all when last is nil).
func write(dev Device, f frame, last *frame) error {
	if last == nil || f.digits != last.digits {
		if err := dev.SetDigitsRaw(f.digits); err != nil {
			return err
		}
	}
	if last == nil || f.matrix != last.matrix {
		if err := dev.SetMatrixRaw(f.matrix); err != nil {
			return err
		}
	}
	if last == nil || f.leds != last.leds {
		if err := dev.WriteReport(f.leds.Report(0)); err != nil {
			return err
		}
	}
	return nil
}

// blank turns every display and LED off (all LED layers, trap 7).
func blank(dev Device) {
	_ = dev.SetDigitsRaw([14]byte{})
	_ = dev.SetMatrixRaw([32]byte{})
	_ = dev.ClearLEDs(-1)
}

// recenterKnobs writes the color knobs' counters back to the middle when the
// controller asks for it, then reads the positions back: they become the
// knobs' references, so nothing jumps even if the panel ignored the write.
func (c *Controller) recenterKnobs(dev Device) {
	rep, ok := c.takeRecenter()
	if !ok {
		return
	}
	if err := dev.SetFeature(rep); err != nil {
		c.log.Debug("panel: knob recenter not written", "err", err)
	}
	b, err := dev.GetFeature(rep[0], len(rep))
	if err != nil {
		c.log.Debug("panel: knob positions not read back", "err", err)
		return
	}
	if pos, ok := micropanel.ParseAnalog(b); ok {
		c.knobsRead(pos)
	}
}
