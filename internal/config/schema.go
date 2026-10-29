package config

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/limanix/client/internal/domain"
)

var byteSizeType = reflect.TypeFor[domain.ByteSize]()

func joinField(parent, name string) string {
	if parent == "" {
		return name
	}

	return parent + "." + name
}

func checkShape(value any, model reflect.Type, field string) error {
	if model == byteSizeType {
		return checkSize(value, field)
	}

	switch model.Kind() {
	case reflect.Struct:
		return checkTable(value, model, field)
	case reflect.Slice:
		return checkArray(value, model, field)
	case reflect.Map:
		return checkEnvironment(value, model, field)
	case reflect.String:
		text, ok := value.(string)
		if !ok {
			return fieldError(field, "expected a string")
		}

		if err := validateString(reflect.ValueOf(text).Convert(model)); err != nil {
			return wrapField(field, err.Error(), err)
		}
	case reflect.Int:
		if _, ok := value.(int64); !ok {
			return fieldError(field, "expected an integer")
		}
	case reflect.Bool:
		if _, ok := value.(bool); !ok {
			return fieldError(field, "expected a boolean")
		}
	default:
		return fieldError(field, "unsupported configuration field type")
	}

	return nil
}

func checkSize(value any, field string) error {
	text, ok := value.(string)
	if !ok {
		return fieldError(field, "expected a whole GiB size string")
	}

	if _, err := domain.ParseByteSize(text); err != nil {
		return wrapField(field, err.Error(), err)
	}

	return nil
}

// checkTable rejects unknown fields before traversing model fields in declaration order.
func checkTable(value any, model reflect.Type, field string) error {
	table, ok := value.(map[string]any)
	if !ok {
		return fieldError(field, "expected a table")
	}

	fields := make(map[string]reflect.StructField, model.NumField())
	for index := range model.NumField() {
		item := model.Field(index)
		fields[item.Tag.Get("toml")] = item
	}

	for _, key := range sortedKeys(table) {
		if _, exists := fields[key]; !exists {
			return fieldError(joinField(field, key), "unknown field")
		}
	}

	for index := range model.NumField() {
		var (
			item          = model.Field(index)
			name          = item.Tag.Get("toml")
			child         = joinField(field, name)
			entry, exists = table[name]
		)

		if !exists {
			if item.Tag.Get("required") == "true" {
				return fieldError(child, "required field is missing")
			}

			continue
		}

		if err := checkShape(entry, item.Type, child); err != nil {
			return err
		}
	}

	return nil
}

func checkArray(value any, model reflect.Type, field string) error {
	entries, ok := value.([]any)
	if !ok {
		return fieldError(field, "expected an array")
	}

	for index, entry := range entries {
		if err := checkShape(entry, model.Elem(), fmt.Sprintf("%s[%d]", field, index)); err != nil {
			return err
		}
	}

	return nil
}

func checkEnvironment(value any, model reflect.Type, field string) error {
	table, ok := value.(map[string]any)
	if !ok {
		return fieldError(field, "expected a table")
	}

	for _, key := range sortedKeys(table) {
		if _, err := domain.NewEnvName(key); err != nil {
			return wrapField(field, err.Error(), err)
		}

		if err := checkShape(table[key], model.Elem(), joinField(field, key)); err != nil {
			return err
		}
	}

	return nil
}

func sortedKeys[V any](table map[string]V) []string {
	keys := make([]string, 0, len(table))

	for key := range table {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

func sortedMapKeys(value reflect.Value) []reflect.Value {
	keys := value.MapKeys()
	sort.Slice(keys, func(a, b int) bool {
		return keys[a].String() < keys[b].String()
	})
	return keys
}
