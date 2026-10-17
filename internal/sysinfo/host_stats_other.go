//go:build !windows

package sysinfo

type platformReader struct{}

func (platformReader) Read() (Counters, error) { return Counters{}, ErrUnsupported }
