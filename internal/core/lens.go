package core

import "math"

// Lens is what a camera reports about its mounted lens, so the GUI can show
// explicit values (f-stops, millimetres, metres) and stay inside the lens'
// limits. A zero field is unknown.
type Lens struct {
	// IrisMin and IrisMax are the widest and the narrowest f-stop.
	IrisMin float64 `json:"irisMin,omitempty"`
	IrisMax float64 `json:"irisMax,omitempty"`
	// FocalLength is the current focal length; FocalMin and FocalMax the zoom
	// range. All in millimetres; a prime lens has FocalMin == FocalMax.
	FocalLength float64 `json:"focalLength,omitempty"`
	FocalMin    float64 `json:"focalMin,omitempty"`
	FocalMax    float64 `json:"focalMax,omitempty"`
	// FocusDistance is the current focus distance; FocusNear and FocusFar the
	// focus range. All in metres. FocusFar is FocusInfinity when the lens
	// focuses to infinity.
	FocusDistance float64 `json:"focusDistance,omitempty"`
	FocusNear     float64 `json:"focusNear,omitempty"`
	FocusFar      float64 `json:"focusFar,omitempty"`
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
