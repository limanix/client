package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDuplicateRecordFieldsAreRejected(t *testing.T) {
	for _, test := range []struct {
		name  string
		field string
		value string
	}{
		{"identity.json", "id", `"fedcba987654"`},
		{"identity.json", `\u0069d`, `"fedcba987654"`},
		{"instance.json", "status", `"failed"`},
		{"instance.json", "schema_version", `999`},
	} {
		t.Run(test.name+":"+test.field, func(t *testing.T) {
			store, original := fixture(t, "sandbox")
			if err := store.Save(original); err != nil {
				t.Fatal(err)
			}
			directory, err := store.InstanceDir(original.Identity.Name)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, test.name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = append([]byte(`{"`+test.field+`":`+test.value+`,`), data[1:]...)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(original.Identity.Name); !errors.Is(err, ErrRecordFields) {
				t.Fatalf("duplicate field not rejected: %v", err)
			}
			entries, err := store.FetchAll()
			if err != nil || len(entries) != 1 || entries[0].Instance != nil || entries[0].Error == nil {
				t.Fatalf("ambiguous record not isolated: %+v %v", entries, err)
			}
			if test.name == "instance.json" && entries[0].Identity == nil {
				t.Fatal("duplicate runtime field hid independent identity")
			}
		})
	}
}
