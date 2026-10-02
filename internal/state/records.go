package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/limanix/client/internal/domain"
	"github.com/limanix/client/internal/filesystem"
	"golang.org/x/sys/unix"
)

const schemaVersion = 1

type identityRecord struct {
	SchemaVersion int `json:"schema_version"`
	domain.Identity
}

type runtimeRecord struct {
	SchemaVersion int           `json:"schema_version"`
	Generation    string        `json:"generation"`
	Status        domain.Status `json:"status"`
	Error         *string       `json:"error"`
}

func decodeIdentityRecord(record identityRecord, name domain.VMName) (domain.Identity, error) {
	if record.SchemaVersion != schemaVersion {
		return domain.Identity{}, ErrUnsupportedSchema
	}

	if record.Name != name {
		return domain.Identity{}, ErrIdentityName
	}

	return record.Normalized()
}

func validateRuntime(record runtimeRecord) error {
	if record.SchemaVersion != schemaVersion {
		return ErrUnsupportedSchema
	}

	if !domain.ValidIdentifier(record.Generation) {
		return ErrInvalidGeneration
	}

	switch record.Status {
	case domain.Creating, domain.Ready, domain.Updating, domain.Failed, domain.Deleting:
		return nil
	case domain.Interrupted:
		return ErrComputedStatus
	default:
		return ErrInvalidStatus
	}
}

func writeRecord(path string, record any) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return filesystem.WriteFileAtomic(path, append(data, '\n'), 0o600)
}

func readRecord(path string, record any) error {
	file, err := filesystem.OpenRegular(path, unix.O_RDONLY, 0)
	if err != nil {
		return err
	}

	data, readErr := io.ReadAll(file)
	if err = errors.Join(readErr, file.Close()); err != nil {
		return err
	}

	if !utf8.Valid(data) {
		return ErrRecordEncoding
	}

	if err = validateRecordFields(data, jsonFieldNames(reflect.TypeOf(record).Elem())); err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(record)
}

// validateRecordFields requires one object with each exact field present once.
// Unmarshaling to a map would silently discard duplicate ownership or status fields.
func validateRecordFields(data []byte, names []string) error {
	remaining := make(map[string]struct{}, len(names))
	for _, name := range names {
		remaining[name] = struct{}{}
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if opening != json.Delim('{') {
		return ErrRecordFields
	}

	for decoder.More() {
		field, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := field.(string)
		if !ok {
			return ErrRecordFields
		}
		if _, exists := remaining[name]; !exists {
			return ErrRecordFields
		}
		delete(remaining, name)

		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}

	if _, err := decoder.Token(); err != nil {
		return err
	}
	if len(remaining) != 0 {
		return ErrRecordFields
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrRecordFields
	}
	return nil
}

func jsonFieldNames(model reflect.Type) []string {
	var result []string

	for index := range model.NumField() {
		field := model.Field(index)
		if field.Anonymous {
			result = append(result, jsonFieldNames(field.Type)...)
			continue
		}

		result = append(result, strings.Split(field.Tag.Get("json"), ",")[0])
	}

	return result
}
