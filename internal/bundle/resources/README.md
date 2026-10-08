# Packaged Lima resources

`cmd/bundle-socketvmnet` runs from `task release/build` or `task ci/golang/test`.
It downloads the pinned macOS helper release for arm64 and amd64.
Taskfile supplies its version, archive digests, sizes, and minimum macOS through linker flags.
A direct invocation without those flags fails.
The original tar.gz archives are embedded with their upstream license.
Packaging verifies their pinned SHA-256 digests, sizes, Mach-O architecture, macOS deployment target, and system-library dependencies.
The supported host minimum comes from `macos_version`.
Runtime installation needs administrator approval.
For dependency update procedures, see the [embedded resources section of the development guide](../../../guides/development.md#embedded-resources).

`go run ./cmd/bundle-guestagent` builds the pinned Lima guest agent for Linux arm64 and amd64.
It writes deterministic gzip archives and an integrity manifest here.
Packaged archives and the manifest are build inputs supplied before CI and release builds.
They are not tracked in Git.

The guest-agent command is declared as a Go tool in `go.mod`.
Dependency updates preserve its Linux-specific package graph.
Generated executables use the same pinned modules as LimaNix.

The generator uses one resolved Go compiler.
It sets `GOENV=off`, `GOWORK=off`, empty `GOFLAGS` and `GOEXPERIMENT`, with CGO disabled and fixed CPU baselines.
Explicit cache, proxy, and authentication environment settings are preserved.

This file keeps the embedded resource directory present in a clean checkout.
The directory embed excludes dot-prefixed leftovers from interrupted writes.
VM creation requires the generated archives.
