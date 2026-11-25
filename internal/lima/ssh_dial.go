package lima

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"golang.org/x/crypto/ssh"
)

const dialTimeout = 10 * time.Second

// Dial opens an SSH connection to a running VM as its management user, with Lima's key and without a host key check,
// as the ssh command of Shell does for the VM's local SSH port.
func (client *Client) Dial(ctx context.Context, name string) (connection *ssh.Client, failure error) {
	defer func() {
		failure = wrapOperation("SSH", failure)
	}()

	inst, err := client.inspectInstance(ctx, name)
	if err != nil {
		return nil, err
	}

	if inst.Status != limatype.StatusRunning {
		return nil, fmt.Errorf("VM %s is not running", name)
	}

	if err = validateSSHMetadata(inst); err != nil {
		return nil, err
	}

	key, err := client.UserKey(ctx)
	if err != nil {
		return nil, err
	}

	pem, err := os.ReadFile(key.PrivateFile)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		return nil, err
	}

	hostKey, algorithms, err := pinnedHostKeyCheck(inst.Dir)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:              *inst.Config.User.Name,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback:   hostKey,
		HostKeyAlgorithms: algorithms,
		Timeout:           dialTimeout,
	}

	return ssh.Dial("tcp", net.JoinHostPort(inst.SSHAddress, strconv.Itoa(inst.SSHLocalPort)), config)
}

// pinnedHostKeyCheck verifies the host key against the key that Lima pinned in ssh.known_hosts on the first boot, as
// the ssh command of Shell does. A VM without that file has never been started by a Lima that pins keys, and Shell
// does not verify it either.
func pinnedHostKeyCheck(instDir string) (ssh.HostKeyCallback, []string, error) {
	path := filepath.Join(instDir, filenames.SSHKnownHosts)

	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return ssh.InsecureIgnoreHostKey(), nil, nil
	} else if err != nil {
		return nil, nil, err
	}

	pinned, err := sshutil.ReadPinnedHostKey(instDir)
	if err != nil {
		return nil, nil, err
	}

	if pinned == nil {
		return nil, nil, fmt.Errorf("%s pins no SSH host key yet; start the VM again", path)
	}

	check := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if bytes.Equal(key.Marshal(), pinned.Marshal()) {
			return nil
		}

		return fmt.Errorf("%w: the SSH host key of the VM (%s) does not match the key pinned in %s (%s)",
			sshutil.ErrHostKeyMismatch, ssh.FingerprintSHA256(key), path, ssh.FingerprintSHA256(pinned))
	}

	return check, hostKeyAlgorithms(pinned), nil
}

// hostKeyAlgorithms makes the server present a key of the pinned type, as it may hold keys of several types.
func hostKeyAlgorithms(pinned ssh.PublicKey) []string {
	if pinned.Type() == ssh.KeyAlgoRSA {
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
	}

	return []string{pinned.Type()}
}
