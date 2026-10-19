package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
)

// Input is one guest flake input: its declared GitHub URL and the name of the lock node that pins it.
type Input struct {
	URL  string
	Node string
}

type lockFile struct {
	Version int                        `json:"version"`
	Root    string                     `json:"root"`
	Nodes   map[string]json.RawMessage `json:"nodes"`
}

type limaPin struct {
	input Input
	nodes map[string]json.RawMessage
}

// readNixosLima accepts the nixos-lima input and its dependency graph, which reaches nixpkgs only by
// following the catalog's own pin.
func readNixosLima(files fs.FS) (limaPin, error) {
	data, err := fs.ReadFile(files, "flake.lock")
	if err != nil {
		return limaPin{}, fmt.Errorf("%w: read flake.lock: the catalog must include its nixos-lima pin: %w", ErrArchive, err)
	}

	pin, err := parseNixosLima(data)
	if err != nil {
		return limaPin{}, fmt.Errorf("%w: flake.lock: %w", ErrArchive, err)
	}
	return pin, nil
}

func parseNixosLima(data []byte) (limaPin, error) {
	var lock lockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return limaPin{}, fmt.Errorf("decode nixos-lima pin: %w", err)
	}

	var root struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	if lock.Root == "" || len(lock.Nodes[lock.Root]) == 0 {
		return limaPin{}, fmt.Errorf("root must identify a lock node")
	}
	if err := json.Unmarshal(lock.Nodes[lock.Root], &root); err != nil {
		return limaPin{}, fmt.Errorf("decode root node: %w", err)
	}
	var name, nixpkgs string
	if err := json.Unmarshal(root.Inputs["nixos-lima"], &name); err != nil || name == "" {
		return limaPin{}, fmt.Errorf("root.inputs.nixos-lima must name a lock node directly")
	}
	if err := json.Unmarshal(root.Inputs["nixpkgs"], &nixpkgs); err != nil || nixpkgs == "" {
		return limaPin{}, fmt.Errorf("root.inputs.nixpkgs must name a lock node directly")
	}
	if len(lock.Nodes[name]) == 0 {
		return limaPin{}, fmt.Errorf("nixos-lima input refers to missing node %q", name)
	}
	fields, err := pinFields(lock.Nodes[name], "inputs", "flake", "locked", "original")
	if err != nil {
		return limaPin{}, fmt.Errorf("nixos-lima node: %w", err)
	}
	if _, err := pinFields(fields["locked"], "type", "owner", "repo", "rev", "narHash", "lastModified"); err != nil {
		return limaPin{}, fmt.Errorf("nixos-lima locked source: %w", err)
	}
	if _, err := pinFields(fields["original"], "type", "owner", "repo", "ref"); err != nil {
		return limaPin{}, fmt.Errorf("nixos-lima original source: %w", err)
	}

	var node struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
		Flake  json.RawMessage            `json:"flake"`
		Locked struct {
			Type    string `json:"type"`
			Owner   string `json:"owner"`
			Repo    string `json:"repo"`
			Rev     string `json:"rev"`
			NarHash string `json:"narHash"`
		} `json:"locked"`
		Original struct {
			Type  string `json:"type"`
			Owner string `json:"owner"`
			Repo  string `json:"repo"`
			Ref   string `json:"ref"`
		} `json:"original"`
	}
	if err := json.Unmarshal(lock.Nodes[name], &node); err != nil {
		return limaPin{}, fmt.Errorf("decode nixos-lima node: %w", err)
	}
	if len(node.Flake) != 0 && !bytes.Equal(bytes.TrimSpace(node.Flake), []byte("true")) {
		return limaPin{}, fmt.Errorf("nixos-lima must be a flake input")
	}
	if node.Locked.Type != "github" || node.Locked.Owner != "nixos-lima" || node.Locked.Repo != "nixos-lima" ||
		node.Original.Type != "github" || node.Original.Owner != "nixos-lima" || node.Original.Repo != "nixos-lima" {
		return limaPin{}, fmt.Errorf("nixos-lima original and locked sources must be github:nixos-lima/nixos-lima")
	}
	if !nixpkgsRefPattern.MatchString(node.Original.Ref) {
		return limaPin{}, fmt.Errorf("nixos-lima original.ref must be a nonempty GitHub ref using letters, digits, '.', '_', '/', or '-'")
	}
	if !nixpkgsRevPattern.MatchString(node.Locked.Rev) {
		return limaPin{}, fmt.Errorf("nixos-lima locked.rev must contain 40 lowercase hexadecimal characters")
	}
	if !sriHash(node.Locked.NarHash) {
		return limaPin{}, fmt.Errorf("nixos-lima locked.narHash must be a SHA-256 SRI hash")
	}
	if !followsPath(node.Inputs["nixpkgs"], "nixpkgs") {
		return limaPin{}, fmt.Errorf("nixos-lima must follow the catalog's nixpkgs input")
	}

	nodes := map[string]json.RawMessage{}
	if err := collectInputs(lock, name, nixpkgs, nodes); err != nil {
		return limaPin{}, err
	}
	return limaPin{input: Input{URL: "github:nixos-lima/nixos-lima/" + node.Original.Ref, Node: name}, nodes: nodes}, nil
}

// collectInputs copies the graph below name. Dependencies name nodes; root inputs are reached by path only.
func collectInputs(lock lockFile, name, nixpkgs string, nodes map[string]json.RawMessage) error {
	if name == lock.Root || name == nixpkgs {
		return fmt.Errorf("nixos-lima graph must reach %q only by following a root input", name)
	}
	if _, visited := nodes[name]; visited {
		return nil
	}
	raw := lock.Nodes[name]
	if len(raw) == 0 {
		return fmt.Errorf("nixos-lima graph refers to missing node %q", name)
	}
	nodes[name] = raw

	var node struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return fmt.Errorf("decode node %q: %w", name, err)
	}
	for input, reference := range node.Inputs {
		var dependency string
		if err := json.Unmarshal(reference, &dependency); err == nil {
			if err := collectInputs(lock, dependency, nixpkgs, nodes); err != nil {
				return err
			}
			continue
		}
		var path []string
		if err := json.Unmarshal(reference, &path); err != nil || len(path) == 0 || (path[0] != "nixpkgs" && path[0] != "nixos-lima") {
			return fmt.Errorf("node %q: input %q must name a node or follow nixpkgs or nixos-lima", name, input)
		}
	}
	return nil
}

func followsPath(reference json.RawMessage, input string) bool {
	var path []string
	return json.Unmarshal(reference, &path) == nil && len(path) == 1 && path[0] == input
}
