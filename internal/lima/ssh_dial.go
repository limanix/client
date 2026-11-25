package lima

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/lima-vm/lima/v2/pkg/limatype"
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

	config := &ssh.ClientConfig{
		User:            *inst.Config.User.Name,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         dialTimeout,
	}

	return ssh.Dial("tcp", net.JoinHostPort(inst.SSHAddress, strconv.Itoa(inst.SSHLocalPort)), config)
}
