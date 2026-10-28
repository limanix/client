package config

import (
	"reflect"
	"strings"

	"github.com/limanix/client/internal/domain"
)

// Field is a key of limanix.toml: its TOML type, the values it accepts, its default as a TOML literal, and its
// description. A table has no default, and an entry field of a table array has one only when the model declares it.
type Field struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"`
	Choices  []string `json:"choices"`
	Default  *string  `json:"default"`
	Required bool     `json:"required"`
	Example  *string  `json:"example"`
	Doc      string   `json:"doc"`
}

// Reference lists every key of limanix.toml from the model's types, tags and defaults. The guest architecture has no
// fixed default: it is the Mac's.
func Reference() ([]Field, error) {
	var fields []Field
	if err := referenceFields(&fields, reflect.ValueOf(Default()), ""); err != nil {
		return nil, err
	}

	return fields, nil
}

func referenceFields(fields *[]Field, model reflect.Value, prefix string) error {
	for index := range model.NumField() {
		var (
			item  = model.Type().Field(index)
			value = model.Field(index)
			key   = joinField(prefix, item.Tag.Get("toml"))
		)

		switch {
		case value.Kind() == reflect.Struct:
			*fields = append(*fields, Field{Key: key, Type: "table", Choices: []string{}, Doc: item.Tag.Get("doc")})
			if err := referenceFields(fields, value, key); err != nil {
				return err
			}
		case isTableArray(value):
			empty := "[]"
			*fields = append(*fields, Field{
				Key: key, Type: "array of tables", Choices: []string{}, Default: &empty, Doc: item.Tag.Get("doc"),
			})
			entry := value.Type().Elem()
			for field := range entry.NumField() {
				*fields = append(*fields, entryField(key+"[]", entry.Field(field)))
			}
		default:
			field := valueField(key, item)
			if item.Type != reflect.TypeFor[domain.Architecture]() {
				encoded, err := defaultLiteral(value)
				if err != nil {
					return err
				}
				field.Default = &encoded
			}
			*fields = append(*fields, field)
		}
	}

	return nil
}

func valueField(key string, item reflect.StructField) Field {
	field := Field{
		Key:     key,
		Type:    modelTypeName(item.Type),
		Choices: choices(item),
		Doc:     item.Tag.Get("doc"),
	}
	if example, ok := item.Tag.Lookup("example"); ok {
		field.Example = &example
	}

	return field
}

func entryField(prefix string, item reflect.StructField) Field {
	field := valueField(joinField(prefix, item.Tag.Get("toml")), item)
	field.Required = item.Tag.Get("required") == "true"
	if declared, ok := item.Tag.Lookup("default"); ok {
		quoted := quoteTOMLString(declared)
		field.Default = &quoted
	}

	return field
}

func choices(item reflect.StructField) []string {
	if item.Type == reflect.TypeFor[domain.Architecture]() {
		values := domain.Architectures()
		names := make([]string, len(values))
		for index, value := range values {
			names[index] = string(value)
		}

		return names
	}

	if listed := item.Tag.Get("choices"); listed != "" {
		return strings.Split(listed, ",")
	}

	return []string{}
}

func defaultLiteral(value reflect.Value) (string, error) {
	if value.Kind() != reflect.Map {
		return tomlLiteral(value)
	}

	entries := make([]string, 0, value.Len())
	for _, key := range sortedMapKeys(value) {
		encoded, err := tomlLiteral(value.MapIndex(key))
		if err != nil {
			return "", err
		}
		entries = append(entries, key.String()+" = "+encoded)
	}

	if len(entries) == 0 {
		return "{}", nil
	}

	return "{ " + strings.Join(entries, ", ") + " }", nil
}

func modelTypeName(model reflect.Type) string {
	if model == byteSizeType {
		return "string (GiB)"
	}

	switch model.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int:
		return "integer"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice:
		return "array[" + modelTypeName(model.Elem()) + "]"
	case reflect.Map:
		return "table[" + modelTypeName(model.Key()) + ", " + modelTypeName(model.Elem()) + "]"
	default:
		return model.Name()
	}
}
