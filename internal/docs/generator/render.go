package generator

import (
	"encoding/json"
	"fmt"

	"github.com/mr-chelyshkin/limanix/internal/buildinfo"
	"github.com/mr-chelyshkin/limanix/internal/cli"
	"github.com/mr-chelyshkin/limanix/internal/config"
)

type document struct {
	path    string
	content []byte
}

func render() ([]document, error) {
	example, err := config.RenderExample()
	if err != nil {
		return nil, fmt.Errorf("render configuration example: %w", err)
	}

	configuration, err := config.RenderReference()
	if err != nil {
		return nil, fmt.Errorf("render configuration reference: %w", err)
	}

	metadata, err := json.Marshal(struct {
		Version string `json:"version"`
	}{Version: buildinfo.Version})
	if err != nil {
		return nil, fmt.Errorf("encode version metadata: %w", err)
	}

	return []document{
		{
			path:    "build/docs/generated/limanix.example.toml",
			content: example,
		},
		{
			path:    "build/docs/generated/configuration.md",
			content: []byte(configuration),
		},
		{
			path:    "build/docs/generated/cli.md",
			content: []byte(cli.Reference()),
		},
		{
			path:    "build/docs/generated/metadata.json",
			content: append(metadata, '\n'),
		},
	}, nil
}
