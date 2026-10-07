package guest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/limanix/client/internal/lima"
)

// contractVersion is the version of the lmx host contract that this client reads.
const contractVersion = 1

// Error codes of the host contract that change what the host does; any other code is a plain failure.
const (
	codeBuildFailed    = "apply.build_failed"
	codeDiskLow        = "disk.low"
	codeFinalizeFailed = "finalize.failed"
)

var errNoAnswer = errors.New("lmx gave no answer of the host contract")

// Error is an error answer of lmx: Code is for programs, Message for people.
type Error struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details"`
}

// Error returns the message of lmx, which already explains the failure.
func (err *Error) Error() string {
	return "lmx: " + err.Message
}

type envelope struct {
	Contract *int            `json:"contract"`
	OK       *bool           `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *Error          `json:"error"`
}

// lmx runs `sudo COMMAND --json` in the guest and decodes the data of the answer into data, which may be nil.
func (guest *Guest) lmx(ctx context.Context, name string, data any, command ...string) error {
	args := append(append([]string{"sudo"}, command...), "--json")
	output, err := guest.client.Run(ctx, name, args, true)

	return decodeAnswer([]byte(output), err, data)
}

// decodeAnswer decodes the answer lmx wrote to standard output. The answer wins over the failure of the command,
// because lmx exits with status 1 after an error answer; output without an answer fails with the command's error.
func decodeAnswer(output []byte, failure error, data any) error {
	err := decodeEnvelope(bytes.TrimSpace(output), data)
	if errors.Is(err, errNoAnswer) && failure != nil {
		return failure
	}

	return err
}

// decodeEnvelope checks the contract version and that ok comes with data or an error before it reads data.
func decodeEnvelope(output []byte, data any) error {
	var answer envelope
	if json.Unmarshal(output, &answer) != nil || answer.Contract == nil || answer.OK == nil {
		return errNoAnswer
	}

	switch {
	case *answer.Contract != contractVersion:
		return fmt.Errorf("lmx answered with host contract %d; this LimaNix reads contract %d", *answer.Contract, contractVersion)
	case !*answer.OK && answer.Error == nil:
		return errNoAnswer
	case !*answer.OK:
		return answer.Error
	case len(answer.Data) == 0 || string(answer.Data) == "null":
		return errNoAnswer
	case data == nil:
		return nil
	}

	if err := json.Unmarshal(answer.Data, data); err != nil {
		return fmt.Errorf("decode the answer of lmx: %w", err)
	}

	return nil
}

// missingLMX reports a guest whose platform predates lmx: sudo cannot find the command, or the older lmx shell
// command rejects the arguments with a usage error.
func missingLMX(err error) bool {
	failure, ok := errors.AsType[*lima.CommandError](err)
	if !ok {
		return false
	}

	usage := failure.Exit != nil && failure.Exit.ExitCode() == 2

	return usage || strings.Contains(failure.Detail, "command not found")
}
