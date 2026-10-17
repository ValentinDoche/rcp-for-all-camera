package panel

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel/panelmodel"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// echoGuard is how long after a lever/ring write the camera's reports are
// taken as the echo of what the panel sent. A camera answers late: moved
// fast, the lever sends 4.0 then 4.5 and the camera reports 4.0 after 4.5
// was sent. It also snaps the iris to the lens' stops and stops at the
// lens' ends (asked f/2.0, a f/2.8 lens stays at 2.8).
const echoGuard = 800 * time.Millisecond

// echoTol is how far (in lever travel, 0..1) outside the span of the recent
// writes a report may land and still be their echo (a snapped stop).
const echoTol = 0.08

// recentWrites are the values a lever or ring sent lately.
type recentWrites struct {
	at     time.Time // last write
	lo, hi float64   // span of the values sent since the guard last expired
}

func (r *recentWrites) add(v float64, now time.Time) {
	if r.at.IsZero() || now.Sub(r.at) > echoGuard {
		r.lo, r.hi = v, v
	} else {
		r.lo, r.hi = min(r.lo, v), max(r.hi, v)
	}
	r.at = now
}

// guarding reports whether reports are still expected from the writes.
func (r recentWrites) guarding(now time.Time) bool {
	return !r.at.IsZero() && now.Sub(r.at) <= echoGuard
}

// noteWriteLocked records a lever/ring write of v to control id.
func (c *Controller) noteWriteLocked(id core.ControlID, v float64) {
	c.written[id] = v
	delete(c.late, id) // an echo of an older write is stale now
	r := c.recent[id]
	r.add(v, c.now())
	c.recent[id] = r
}

// echoLocked reports whether a camera report of v for the lever/ring
// control id is the echo of the panel's own recent writes: it arrives
// within the guard and lands inside (or just beside) the span of the values
// sent, measured on the lever's travel. An echo neither releases the lever
// nor replaces the value the panel sent; it is kept and adopted once the
// camera has settled (settleLocked), so a snapped or clamped value shows
// without the lever losing control.
func (c *Controller) echoLocked(id core.ControlID, v float64) bool {
	r, ok := c.recent[id]
	if !ok || !r.guarding(c.now()) {
		return false
	}
	spec, _, declared, _ := c.cam.Get(id)
	if !declared {
		return false
	}
	var pos func(float64) float64
	switch id {
	case core.Iris:
		w := c.irisWindowLocked(spec)
		pos = func(x float64) float64 { return panelmodel.IrisPos(spec, w, x) }
	case core.IrisNorm:
		w := c.normWindowLocked(spec)
		pos = func(x float64) float64 { return panelmodel.NormPos(spec, w, x) }
	case core.MasterBlack:
		w := panelmodel.MBWindow(spec)
		pos = func(x float64) float64 { return panelmodel.MBPos(spec, w, x) }
	default:
		return false
	}
	p, a, b := pos(v), pos(r.lo), pos(r.hi)
	lo, hi := min(a, b)-echoTol, max(a, b)+echoTol
	if p < lo || p > hi {
		return false
	}
	c.late[id] = v
	return true
}

// settleLocked adopts the camera's last echoed value once no write is
// pending any more: the displays then show what the camera really did, and
// the lever keeps control.
func (c *Controller) settleLocked(now time.Time) {
	for id, v := range c.late {
		if r := c.recent[id]; r.guarding(now) {
			continue
		}
		delete(c.late, id)
		if _, ok := c.cam.Values[id]; ok {
			c.cam.Values[id] = core.Number(v)
			c.written[id] = v
		}
	}
}
