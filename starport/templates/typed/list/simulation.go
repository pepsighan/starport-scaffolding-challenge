package list

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/placeholder"
	"github.com/tendermint/starport/starport/templates/typed"
)

func moduleSimulationModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "module_simulation.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Create a list of two different indexes and fields to use as sample
		msgField := fmt.Sprintf("%s: sample.AccAddress(),\n", opts.MsgSigner.UpperCamel)

		content := f.String()
		// simulation genesis state
		templateGs := `%[1]vList: []types.%[1]v{
		{
			Id: 0,
			%[2]v
		},
		{
			Id: 1,
			%[2]v
		},
	},
	%[1]vCount: 2`
		genesisStateSnippet := fmt.Sprintf(
			templateGs,
			opts.TypeName.UpperCamel,
			msgField,
		)
		if strings.Count(content, typed.PlaceholderSimappGenesisState) != 0 {
			// Use the older placeholder mechanism for older codebase.
			genesisStateSnippet += ",\n" + typed.PlaceholderSimappGenesisState
			content = replacer.Replace(content, typed.PlaceholderSimappGenesisState, genesisStateSnippet)
		} else {
			// Use the clipper based code generation for newer codebase.
			content, err = clipper.PasteGoReturningCompositeNewArgumentSnippetAt(
				path,
				content,
				genesisStateSnippet,
				clipper.SelectOptions{
					"functionName": "newGenesisState",
				},
			)
			if err != nil {
				return err
			}
		}

		content, err = typed.ModuleSimulationMsgModify(
			replacer,
			path,
			content,
			opts.ModuleName,
			opts.TypeName,
			"Create", "Update", "Delete",
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
