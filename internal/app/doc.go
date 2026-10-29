// Package app assembles the host services used by Limanix commands.
//
// [New] retains command streams without opening state or contacting Lima.
// Each Services instance opens its state store once, when a service is first requested.
// [Services.Manager] checks the host architecture and connects the VM backend, guest access, modules, and managed homes.
// [Services.Registry] opens module services without initializing Lima.
// Help, version output, and generated command references need neither service.
package app
