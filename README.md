[![Release](https://img.shields.io/github/v/release/limanix/client)](https://github.com/limanix/client/releases)
[![License: Apache-2.0](https://img.shields.io/github/license/limanix/client?label=license)](LICENSE)

<div align="center">
<img src=".github/assets/readme-header.png" alt="LimaNix" width="100%">

<p align="center">
<a href="https://docs.limanix.dev/quick-start"><code>→ Get started</code></a>
<a href="https://docs.limanix.dev/"><code>Documentation</code></a>
<a href="https://docs.limanix.dev/contributing/"><code>Contributing</code></a>
</p>

<p align="center"><sub>macOS 26 or newer | Apple Silicon and Intel</sub></p>
</div>

## macOS client

`limanix`, the command-line client for [LimaNix](https://limanix.dev). Create
and manage NixOS development VMs from one `limanix.toml`, with Lima built in.

- **Configure your environment.** Choose VM resources, shell components,
  language toolchains and services in `limanix.toml`.
- **Manage VMs.** Create, update, start and stop environments. Deleting a VM
  preserves its managed home on the Mac by default.
- **Work in the guest.** Open a shell, run commands or attach to a named
  persistent session when a session provider is configured.
- **Diagnose problems.** Check VM health with `doctor` and guest port
  connectivity with `network check`.

Follow the [quick start](https://docs.limanix.dev/quick-start), then explore the
[CLI reference](https://docs.limanix.dev/reference/cli) and
[configuration reference](https://docs.limanix.dev/reference/config). Download
the client from [GitHub Releases](https://github.com/limanix/client/releases).

## Development

> Requires [Task](https://taskfile.dev) 3.53.1+ and Docker. Native builds also
> require Go and the Xcode Command Line Tools.

Run Go tests with `task ci/golang/test modules_version="<catalog-tag>"`; use
`task -l` to list checks and build commands.

Check Markdown with `task ci/markdown/fmt`; format it with `task markdown/fix`.

## Contributing

See the [contribution guide](https://docs.limanix.dev/contributing/) for issues,
pull requests and releases.
