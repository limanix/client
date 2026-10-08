package buildinfo

import "errors"

var (
	ErrMissingPlatform = errors.New("minimum macOS version is not configured; build with task release/build")
	ErrMacOSVersion    = errors.New("invalid macOS version")
)
