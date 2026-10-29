// Package lima adapts Lima's native Go runtime to Limanix operations.
//
// [Client] restricts operations to Limanix names and translates upstream metadata into [Instance].
// It calls Lima libraries directly and uses system SSH for guest commands.
// [NewClient] supplies native APIs and command streams; the zero value is not usable.
// Launching a VM requires a guest-agent provider.
//
// # Host and configuration
//
// [Client.Preflight] checks host requirements without starting a VM.
// Missing QEMU network setup is delegated to package vmnet and requires administrator approval.
// [HostArchitecture] detects Apple Silicon under Rosetta; [RequireNativeArchitecture] rejects translated VM operations.
// [RequireMacOS] checks the host minimum for VM operations and network setup.
//
// [Render] emits JSON accepted as Lima YAML with pinned image URLs and digests.
// Matching host and guest architectures use VZ, virtiofs, and VZ NAT.
// Cross-architecture guests use QEMU, 9p, and Lima's shared network.
// Home, generation, and user mounts are separate; environment values are excluded from the template.
//
// # Execution
//
// Start, stop, and delete serialize network transitions for Limanix processes sharing LIMA_HOME.
// The lock includes image preparation and startup and ends before guest provisioning.
// It does not coordinate external limactl processes or separate Lima homes.
// Separate homes can share socket and PID paths; one home's reconciler can stop another home's network helper.
// Lima starts helpers on demand and stops them after the last user.
//
// [Client.Run] preserves cancellation and bounded stderr in [CommandError].
// Captured output is returned even when the command fails; [Client.Stream] writes output as it arrives.
// [Client.Shell] inherits terminal streams and preserves the remote exit status.
// CommandError excludes command arguments, but guest stderr may contain private data.
package lima
