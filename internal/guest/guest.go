package guest

import (
	"context"
	"io"
	"log"
	"os"
)

// Client is the management connection needed for guest operations.
type Client interface {
	Run(context.Context, string, []string, bool) (string, error)
	Stream(context.Context, string, []string, io.Writer) error
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Shell(context.Context, string, []string) (int, error)
}

// Guest uses a management connection to provision and access the regular guest account.
type Guest struct {
	// Stdout and Stderr receive the build lines of an apply, each on the stream the build wrote it to.
	Stdout io.Writer
	Stderr io.Writer

	// Warn receives problems that do not stop an apply, such as a nearly full disk.
	Warn func(string, ...any)

	client Client
}

// New binds guest operations to the supplied management connection.
func New(client Client) *Guest {
	return &Guest{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Warn:   log.New(os.Stderr, "limanix: warning: ", 0).Printf,
		client: client,
	}
}
