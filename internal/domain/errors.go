package domain

import "errors"

var (
	// ErrInsufficientPrivilege means live capture cannot open the capture device.
	ErrInsufficientPrivilege = errors.New("insufficient privilege for live capture (need root or CAP_NET_RAW)")
	// ErrNoAdapters means no suitable network interfaces were found.
	ErrNoAdapters = errors.New("no network adapters available")
	// ErrCaptureFailed is a generic capture start failure.
	ErrCaptureFailed = errors.New("capture failed")
)
