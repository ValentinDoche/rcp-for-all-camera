package main

import (
	"fmt"
	"math"
)

// fakeCamera is a simulated camera driven by the panel, used to give the
// panel displays realistic content without any real camera attached.
type fakeCamera struct {
	ND          int // index into ndSteps
	CC          int // index into ccSteps
	GainDb      int
	Shutter     int // index into shutterSteps
	Iris        float64
	MasterBlack int // -99..99
	WbK         int
	White       [3]float64 // -1..1
	Black       [3]float64 // -1..1
	AutoIris    bool
	Bars        bool
	On          bool
}

var (
	ndSteps      = []string{"CLEAR", "1/4", "1/16", "1/64"}
	ccSteps      = []string{"A", "b", "C", "d", "E"}
	shutterSteps = []int{24, 25, 30, 50, 60, 100, 120, 250, 500, 1000, 2000}
)

const (
	irisMin = 1.8
	irisMax = 22.0
)

func newFakeCamera(n int) *fakeCamera {
	return &fakeCamera{ND: n % len(ndSteps), CC: 0, GainDb: 0, Shutter: 3, Iris: 2.8 + float64(n%4), WbK: 5600}
}

func clamp[T int | float64](v, lo, hi T) T { return min(max(v, lo), hi) }

func (c *fakeCamera) step(id string) {
	switch id {
	case "ND_UP":
		c.ND = clamp(c.ND+1, 0, len(ndSteps)-1)
	case "ND_DOWN":
		c.ND = clamp(c.ND-1, 0, len(ndSteps)-1)
	case "CC_UP":
		c.CC = clamp(c.CC+1, 0, len(ccSteps)-1)
	case "CC_DOWN":
		c.CC = clamp(c.CC-1, 0, len(ccSteps)-1)
	case "GAIN_UP":
		c.GainDb = clamp(c.GainDb+3, -6, 36)
	case "GAIN_DOWN":
		c.GainDb = clamp(c.GainDb-3, -6, 36)
	case "SHUTTER_UP":
		c.Shutter = clamp(c.Shutter+1, 0, len(shutterSteps)-1)
	case "SHUTTER_DOWN":
		c.Shutter = clamp(c.Shutter-1, 0, len(shutterSteps)-1)
	case "WB":
		switch c.WbK {
		case 3200:
			c.WbK = 4300
		case 4300:
			c.WbK = 5600
		default:
			c.WbK = 3200
		}
	case "AUTO_IRIS":
		c.AutoIris = !c.AutoIris
	case "BARS":
		c.Bars = !c.Bars
	case "ON":
		c.On = !c.On
	}
}

// setIrisFromLever maps the lever (-4096 closed .. +4096 open) to an f-stop on
// a log scale, as a real lens would.
func (c *fakeCamera) setIrisFromLever(lever int32) {
	t := (float64(lever) + 4096) / 8192 // 0 closed .. 1 open
	t = clamp(t, 0, 1)
	c.Iris = irisMax * math.Pow(irisMin/irisMax, t)
}

func (c *fakeCamera) irisText() string {
	if c.Iris < 10 {
		return fmt.Sprintf("F%.1f", c.Iris)
	}
	return fmt.Sprintf("F%.0f", c.Iris)
}

func (c *fakeCamera) shutterText() string { return fmt.Sprintf("1/%d", shutterSteps[c.Shutter]) }

// Panel display strings (sized for the panel digits).
func (c *fakeCamera) ndDigit() string { return fmt.Sprint(c.ND) }
func (c *fakeCamera) gainText() string {
	if c.GainDb < 0 {
		return fmt.Sprintf("-%d", -c.GainDb)
	}
	return fmt.Sprintf("%02d", c.GainDb)
}
func (c *fakeCamera) shutterDigits() string { return fmt.Sprint(shutterSteps[c.Shutter]) }
func (c *fakeCamera) mbText() string {
	if c.MasterBlack < 0 {
		return fmt.Sprintf("-%02d", -c.MasterBlack)
	}
	return fmt.Sprintf("%3d", c.MasterBlack)
}
