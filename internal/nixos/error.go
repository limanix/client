package nixos

import "errors"

var (
	// ErrInvalidUID rejects a root or invalid host UID for the development user.
	ErrInvalidUID = errors.New("guest UID must be positive")

	// ErrInvalidAuthorizedKey rejects an empty SSH key or one that spans lines.
	ErrInvalidAuthorizedKey = errors.New("an authorized SSH key must be one nonempty line")

	// ErrInvalidGeneration rejects a generation ID that the guest could not report back.
	ErrInvalidGeneration = errors.New("generation ID must be 12 lowercase hexadecimal characters")

	// ErrBundleExists prevents overwriting an existing generation's flake inputs.
	ErrBundleExists = errors.New("NixOS bundle already exists")

	// ErrImagePin reports a base image pin without a disk for the guest architecture.
	ErrImagePin = errors.New("base image pin has no disk for this architecture")
)
