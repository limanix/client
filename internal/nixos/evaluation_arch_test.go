package nixos

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/limanix/client/internal/domain"
)

func evaluationArchitectures(value string, supplied bool) ([]domain.Architecture, error) {
	if !supplied {
		return domain.Architectures(), nil
	}
	var selected []domain.Architecture
	for _, name := range strings.Fields(value) {
		arch, err := domain.NewArchitecture(name)
		if err != nil {
			return nil, fmt.Errorf("unsupported architecture %q: %w", name, err)
		}
		if !slices.Contains(selected, arch) {
			selected = append(selected, arch)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("select at least one guest architecture")
	}
	return selected, nil
}

func TestEvaluationArchitectures(t *testing.T) {
	for _, test := range []struct {
		name, value string
		supplied    bool
		want        []domain.Architecture
		invalid     bool
	}{
		{name: "all", want: domain.Architectures()},
		{name: "one", supplied: true, value: "arm64", want: []domain.Architecture{domain.ARM64}},
		{name: "whitespace and duplicates", supplied: true, value: " amd64\narm64\tamd64 ", want: []domain.Architecture{domain.AMD64, domain.ARM64}},
		{name: "empty", supplied: true, invalid: true},
		{name: "unknown", supplied: true, value: "x86_64", invalid: true},
		{name: "partly unknown", supplied: true, value: "arm64 unknown", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := evaluationArchitectures(test.value, test.supplied)
			if test.invalid {
				if err == nil || got != nil {
					t.Fatalf("invalid architecture filter accepted: %v, %v", got, err)
				}
				return
			}
			if err != nil || !slices.Equal(got, test.want) {
				t.Fatalf("architectures: got %v, want %v, error %v", got, test.want, err)
			}
		})
	}
}
