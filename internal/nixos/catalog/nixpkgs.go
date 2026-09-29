package catalog

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

var (
	nixpkgsRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	nixpkgsRevPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type nixpkgsPin struct {
	url  string
	node json.RawMessage
}

// readNixpkgs accepts the self-contained GitHub input consumed by the guest flake.
func readNixpkgs(files fs.FS) (nixpkgsPin, error) {
	data, err := fs.ReadFile(files, "flake.lock")
	if err != nil {
		return nixpkgsPin{}, fmt.Errorf("%w: read flake.lock: the catalog must include its NixOS pin: %w", ErrArchive, err)
	}

	pin, err := parseNixpkgs(data)
	if err != nil {
		return nixpkgsPin{}, fmt.Errorf("%w: flake.lock: %w", ErrArchive, err)
	}
	return pin, nil
}

func parseNixpkgs(data []byte) (nixpkgsPin, error) {
	var lock struct {
		Version int                        `json:"version"`
		Root    string                     `json:"root"`
		Nodes   map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nixpkgsPin{}, fmt.Errorf("decode NixOS pin: %w", err)
	}
	if lock.Version != 7 {
		return nixpkgsPin{}, fmt.Errorf("expected lock version 7, got %d", lock.Version)
	}

	var root struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	if lock.Root == "" || len(lock.Nodes[lock.Root]) == 0 {
		return nixpkgsPin{}, fmt.Errorf("root must identify a lock node")
	}
	if err := json.Unmarshal(lock.Nodes[lock.Root], &root); err != nil {
		return nixpkgsPin{}, fmt.Errorf("decode root node: %w", err)
	}
	var name string
	if err := json.Unmarshal(root.Inputs["nixpkgs"], &name); err != nil || name == "" {
		return nixpkgsPin{}, fmt.Errorf("root.inputs.nixpkgs must name a lock node directly")
	}
	if len(lock.Nodes[name]) == 0 {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs input refers to missing node %q", name)
	}
	fields, err := pinFields(lock.Nodes[name], "inputs", "flake", "locked", "original")
	if err != nil {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs node: %w", err)
	}
	lockedFields, err := pinFields(fields["locked"], "type", "owner", "repo", "rev", "narHash", "lastModified")
	if err != nil {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs locked source: %w", err)
	}
	if _, err := pinFields(fields["original"], "type", "owner", "repo", "ref"); err != nil {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs original source: %w", err)
	}

	var node struct {
		Inputs json.RawMessage `json:"inputs"`
		Flake  json.RawMessage `json:"flake"`
		Locked struct {
			Type         string  `json:"type"`
			Owner        string  `json:"owner"`
			Repo         string  `json:"repo"`
			Rev          string  `json:"rev"`
			NarHash      string  `json:"narHash"`
			LastModified *uint64 `json:"lastModified"`
		} `json:"locked"`
		Original struct {
			Type  string `json:"type"`
			Owner string `json:"owner"`
			Repo  string `json:"repo"`
			Ref   string `json:"ref"`
		} `json:"original"`
	}
	if err := json.Unmarshal(lock.Nodes[name], &node); err != nil {
		return nixpkgsPin{}, fmt.Errorf("decode nixpkgs node: %w", err)
	}
	if _, exists := lockedFields["lastModified"]; exists && node.Locked.LastModified == nil {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs locked.lastModified must be an unsigned integer when present")
	}
	if len(node.Inputs) != 0 {
		var inputs map[string]json.RawMessage
		if err := json.Unmarshal(node.Inputs, &inputs); err != nil || inputs == nil || len(inputs) != 0 {
			return nixpkgsPin{}, fmt.Errorf("nixpkgs inputs must be absent or an empty object")
		}
	}
	if len(node.Flake) != 0 && !bytes.Equal(bytes.TrimSpace(node.Flake), []byte("true")) {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs must be a flake input")
	}
	if node.Locked.Type != "github" || node.Locked.Owner != "NixOS" || node.Locked.Repo != "nixpkgs" ||
		node.Original.Type != "github" || node.Original.Owner != "NixOS" || node.Original.Repo != "nixpkgs" {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs original and locked sources must be github:NixOS/nixpkgs")
	}
	if !nixpkgsRefPattern.MatchString(node.Original.Ref) {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs original.ref must be a nonempty GitHub ref using letters, digits, '.', '_', '/', or '-'")
	}
	if !nixpkgsRevPattern.MatchString(node.Locked.Rev) {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs locked.rev must contain 40 lowercase hexadecimal characters")
	}
	hash, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(node.Locked.NarHash, "sha256-"))
	if !strings.HasPrefix(node.Locked.NarHash, "sha256-") || err != nil || len(hash) != 32 ||
		"sha256-"+base64.StdEncoding.EncodeToString(hash) != node.Locked.NarHash {
		return nixpkgsPin{}, fmt.Errorf("nixpkgs locked.narHash must be a SHA-256 SRI hash")
	}

	return nixpkgsPin{url: "github:NixOS/nixpkgs/" + node.Original.Ref, node: lock.Nodes[name]}, nil
}

// Nix attributes are case-sensitive; encoding/json's struct field matching is not.
func pinFields(data json.RawMessage, allowed ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("expected an object")
	}
	for name := range fields {
		if !slices.Contains(allowed, name) {
			return nil, fmt.Errorf("unsupported attribute %q", name)
		}
	}
	return fields, nil
}
