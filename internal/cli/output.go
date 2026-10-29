package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/limanix/client/internal/domain"
)

func writeJSON[T any](writer io.Writer, entries []T) error {
	if entries == nil {
		entries = []T{}
	}

	return encodeJSON(writer, entries)
}

func encodeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeNames[T any](writer io.Writer, entries []T, name func(T) string) error {
	for _, entry := range entries {
		if _, err := fmt.Fprintln(writer, name(entry)); err != nil {
			return err
		}
	}

	return nil
}

func writeVM(ctx context.Context, writer io.Writer, manager Manager, name domain.VMName) error {
	entry, err := manager.Fetch(ctx, name)
	if err != nil {
		return err
	}

	return encodeJSON(writer, entry)
}

type deletedVM struct {
	Name string  `json:"name"`
	Home *string `json:"home"`
}
