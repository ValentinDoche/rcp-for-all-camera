package core

import "math"

// Lens is what a camera reports about its mounted lens, so the GUI can show
// explicit values (f-stops, millimetres, metres) and stay inside the lens'
// limits. A zero field is unknown.
type Lens struct {
	// IrisMin and IrisMax are the widest and the narrowest f-stop.
	IrisMin float64
	IrisMax float64
	// FocalLength is the current focal length; FocalMin and FocalMax the zoom
	// range. All in millimetres; a prime lens has FocalMin == FocalMax.
	FocalLength float64
	FocalMin    float64
	FocalMax    float64
	// FocusDistance is the current focus distance; FocusNear and FocusFar the
	// focus range. All in metres. FocusFar is FocusInfinity when the lens
	// focuses to infinity.
	FocusDistance float64
	FocusNear     float64
	FocusFar      float64
}

// FocusInfinity stands for a focus distance at infinity. JSON has no
// infinity, so it is a large finite number.
const FocusInfinity = 1e9

// IsInfinity reports whether a focus distance in metres means infinity.
func IsInfinity(m float64) bool { return m >= FocusInfinity || math.IsInf(m, 1) }

// LensReporter is implemented by cameras that know their lens. The camera
// also pushes an Event with Lens set whenever the lens information changes.
type LensReporter interface {
	Lens() Lens
}
