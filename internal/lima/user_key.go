package lima

import (
	"context"
	"path/filepath"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

// UserKey is Lima's SSH key pair, which the development account accepts as well.
type UserKey struct {
	Public      string
	PrivateFile string
}

// UserKey returns Lima's key and creates it on first use.
func (client *Client) UserKey(ctx context.Context) (key UserKey, failure error) {
	defer func() {
		failure = operationError(ctx, "SSH key", failure)
	}()

	keys, err := sshutil.DefaultPubKeys(ctx, false)
	if err != nil {
		return UserKey{}, err
	}

	if len(keys) == 0 || keys[0].Content == "" {
		return UserKey{}, ErrMissingUserKey
	}

	directory, err := dirnames.LimaConfigDir()
	if err != nil {
		return UserKey{}, err
	}

	return UserKey{
		Public:      keys[0].Content,
		PrivateFile: filepath.Join(directory, filenames.UserPrivateKey),
	}, nil
}
