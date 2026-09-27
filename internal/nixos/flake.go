package nixos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/mr-chelyshkin/limanix/internal/filesystem"
	"github.com/mr-chelyshkin/limanix/internal/nixos/catalog"
)

// writeFlake combines the catalog's nixpkgs pin with the client's platform inputs.
// Both inputs are embedded; preparing a generation never resolves a remote ref.
func writeFlake(directory string) error {
	source, err := systemCatalog()
	if err != nil {
		return err
	}
	declaration, lock, err := renderFlake(source)
	if err != nil {
		return err
	}
	if err = filesystem.WriteFileAtomic(filepath.Join(directory, "flake.nix"), declaration, 0o600); err != nil {
		return err
	}
	return filesystem.WriteFileAtomic(filepath.Join(directory, "flake.lock"), lock, 0o600)
}

func renderFlake(source *catalog.Catalog) ([]byte, []byte, error) {
	declaration, err := resources.ReadFile("resources/flake.nix.tmpl")
	if err != nil {
		return nil, nil, err
	}
	parsed, err := template.New("flake.nix").Option("missingkey=error").Parse(string(declaration))
	if err != nil {
		return nil, nil, fmt.Errorf("parse guest flake template: %w", err)
	}
	url, node := source.Nixpkgs()
	var rendered bytes.Buffer
	if err = parsed.Execute(&rendered, struct{ NixpkgsURL string }{url}); err != nil {
		return nil, nil, fmt.Errorf("render guest flake: %w", err)
	}

	data, err := resources.ReadFile("resources/flake.lock.tmpl")
	if err != nil {
		return nil, nil, err
	}
	var lock struct {
		Nodes   map[string]json.RawMessage `json:"nodes"`
		Root    string                     `json:"root"`
		Version int                        `json:"version"`
	}
	if err = json.Unmarshal(data, &lock); err != nil {
		return nil, nil, fmt.Errorf("read platform lock template: %w", err)
	}
	if lock.Nodes == nil || lock.Nodes["nixpkgs"] != nil {
		return nil, nil, fmt.Errorf("platform lock template must leave nixpkgs to the module catalog")
	}
	lock.Nodes["nixpkgs"] = node
	data, err = json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("render guest flake lock: %w", err)
	}
	return rendered.Bytes(), append(data, '\n'), nil
}
