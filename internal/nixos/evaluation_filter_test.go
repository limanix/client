package nixos

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func parseEvaluationFilter(value string, supplied bool, names []string) (map[string]bool, error) {
	available := make(map[string]bool, len(names))
	for _, name := range names {
		available[name] = true
	}
	if !supplied {
		return available, nil
	}
	selected := strings.Fields(value)
	if len(selected) == 0 {
		return nil, fmt.Errorf("select at least one of: %s", strings.Join(names, ", "))
	}
	result := make(map[string]bool, len(selected))
	for _, name := range selected {
		if !available[name] {
			return nil, fmt.Errorf("unknown selection %q; choose from: %s", name, strings.Join(names, ", "))
		}
		result[name] = true
	}
	return result, nil
}

func TestEvaluationFilter(t *testing.T) {
	names := []string{"empty", "console-version", "third-party-capability"}
	for _, test := range []struct {
		name, value string
		supplied    bool
		want        map[string]bool
		invalid     bool
	}{
		{name: "all", want: map[string]bool{"empty": true, "console-version": true, "third-party-capability": true}},
		{name: "one", value: "empty", supplied: true, want: map[string]bool{"empty": true}},
		{name: "whitespace", value: "  empty\nthird-party-capability\t", supplied: true, want: map[string]bool{"empty": true, "third-party-capability": true}},
		{name: "duplicate", value: "empty empty", supplied: true, want: map[string]bool{"empty": true}},
		{name: "empty", supplied: true, invalid: true},
		{name: "unknown", value: "console", supplied: true, invalid: true},
		{name: "partly unknown", value: "empty unknown", supplied: true, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseEvaluationFilter(test.value, test.supplied, names)
			if test.invalid {
				if err == nil || got != nil {
					t.Fatalf("invalid filter accepted: %v, %v", got, err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("filter changed: got %v, want %v, error %v", got, test.want, err)
			}
		})
	}
}
