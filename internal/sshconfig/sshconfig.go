// Package sshconfig writes the OpenSSH client settings that reach each VM as NAME.limanix, for editors on the Mac.
package sshconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/limanix/client/internal/filesystem"
)

// Suffix follows the VM name in the host names of the file.
const Suffix = ".limanix"

const header = `# limanix writes this file after create, update, start and delete; changes here are lost.
# Include it at the top of ~/.ssh/config: https://docs.limanix.dev/vms/editors
`

// Host is one VM: its development account behind the SSH port that Lima forwards on the Mac.
type Host struct {
	Name         string
	User         string
	Port         int
	IdentityFile string
}

// Render returns the file with one block per host, in the given order.
func Render(hosts []Host) []byte {
	var text strings.Builder

	text.WriteString(header)

	for _, host := range hosts {
		fmt.Fprintf(&text, "\nHost %s%s\n", host.Name, Suffix)
		fmt.Fprintf(&text, "  HostName 127.0.0.1\n")
		fmt.Fprintf(&text, "  Port %d\n", host.Port)
		fmt.Fprintf(&text, "  User %s\n", host.User)
		fmt.Fprintf(&text, "  IdentityFile \"%s\"\n", host.IdentityFile)
		text.WriteString("  IdentitiesOnly yes\n")
		text.WriteString("  PreferredAuthentications publickey\n")
		text.WriteString("  StrictHostKeyChecking no\n")
		text.WriteString("  UserKnownHostsFile /dev/null\n")
		text.WriteString("  LogLevel ERROR\n")
	}

	return []byte(text.String())
}

// Write replaces the file at path and creates its private directory.
func Write(path string, hosts []Host) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	return filesystem.WriteFileAtomic(path, Render(hosts), 0o600)
}
