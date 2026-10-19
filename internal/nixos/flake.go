package nixos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/limanix/client/internal/filesystem"
	"github.com/limanix/client/internal/nixos/catalog"
)

// writeFlake combines the catalog's interface, platform and pins with the client's flake template.
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
	if err = filesystem.WriteFileAtomic(filepath.Join(directory, "flake.lock"), lock, 0o600); err != nil {
		return err
	}
	if err = filesystem.WriteFileAtomic(filepath.Join(directory, "interface.nix"), source.Interface(), 0o600); err != nil {
		return err
	}
	return filesystem.WriteFileAtomic(filepath.Join(directory, "platform.nix"), source.Platform(), 0o600)
}

// renderFlake declares the catalog's inputs in the guest flake and locks them with the catalog's nodes.
func renderFlake(source *catalog.Catalog) ([]byte, []byte, error) {
	declaration, err := resources.ReadFile("resources/flake.nix.tmpl")
	if err != nil {
		return nil, nil, err
	}
	parsed, err := template.New("flake.nix").Option("missingkey=error").Parse(string(declaration))
	if err != nil {
		return nil, nil, fmt.Errorf("parse guest flake template: %w", err)
	}
	nixpkgsURL, nixpkgs := source.Nixpkgs()
	lima, nodes := source.NixosLima()
	var rendered bytes.Buffer
	if err = parsed.Execute(&rendered, struct{ NixpkgsURL, NixosLimaURL string }{nixpkgsURL, lima.URL}); err != nil {
		return nil, nil, fmt.Errorf("render guest flake: %w", err)
	}

	for _, name := range []string{"nixpkgs", "root"} {
		if _, taken := nodes[name]; taken {
			return nil, nil, fmt.Errorf("render guest flake lock: the nixos-lima graph owns a node named %q", name)
		}
	}
	nodes["nixpkgs"] = nixpkgs
	root, err := json.Marshal(map[string]map[string]string{"inputs": {"nixpkgs": "nixpkgs", "nixos-lima": lima.Node}})
	if err != nil {
		return nil, nil, err
	}
	nodes["root"] = root
	lock := struct {
		Nodes   map[string]json.RawMessage `json:"nodes"`
		Root    string                     `json:"root"`
		Version int                        `json:"version"`
	}{nodes, "root", 7}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("render guest flake lock: %w", err)
	}
	return rendered.Bytes(), append(data, '\n'), nil
}
