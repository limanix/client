package nixos

import (
	"reflect"
	"strings"
	"testing"

	"github.com/limanix/client/internal/config"
	"github.com/limanix/client/internal/nixos/catalog"
)

// Every shell and toolchain choice that the client offers names a module of the matching kind in the embedded
// catalog, so a client never ships a preset that its catalog cannot build.
func TestShellAndToolchainChoicesExistInTheEmbeddedCatalog(t *testing.T) {
	system, err := SystemModules()
	if err != nil {
		t.Fatal(err)
	}

	var shells []config.Shell
	for _, editor := range []string{"astronvim", "neovim", config.None} {
		for _, multiplexer := range []string{"tmux", config.None} {
			shells = append(shells,
				config.Shell{Preset: config.PresetTerminal, Prompt: "pure", Editor: editor, Multiplexer: multiplexer},
				config.Shell{Preset: config.PresetMinimal, Prompt: config.None, Editor: editor, Multiplexer: multiplexer},
			)
		}
	}

	// The example of each [toolchains] key, which the rendered configuration suggests, is a line of the catalog.
	toolchains := config.Toolchains{}
	model := reflect.TypeFor[config.Toolchains]()
	examples := reflect.ValueOf(&toolchains).Elem()
	for index := range model.NumField() {
		examples.Field(index).SetString(model.Field(index).Tag.Get("example"))
	}

	for _, shell := range shells {
		cfg := config.Default()
		cfg.Shell = shell
		cfg.Toolchains = toolchains
		for _, choice := range cfg.Selection().Modules {
			entry, exists := system[choice.ID.Selector()]
			kind := catalog.KindShell
			if strings.HasPrefix(choice.Field, "toolchains.") {
				kind = catalog.KindToolchains
			}
			if !exists || entry.Kind != kind {
				t.Fatalf("%s chooses %s, which the embedded catalog lacks as a %s module: %+v", choice.Field, choice.ID, kind, entry)
			}
		}
	}
}
