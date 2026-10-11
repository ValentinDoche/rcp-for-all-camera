//go:build !windows

package sysinfo

type platformReader struct{}

func (platformReader) read() (raw, error) { return raw{}, ErrUnsupported }
