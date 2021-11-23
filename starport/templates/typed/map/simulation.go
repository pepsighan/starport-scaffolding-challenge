package maptype

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
		sampleIndexes := make([]string, 2)
		for i := 0; i < 2; i++ {
			sampleIndexes[i] = fmt.Sprintf("%s: sample.AccAddress(),\n", opts.MsgSigner.UpperCamel)
			for _, index := range opts.Indexes {
				sampleIndexes[i] += index.GenesisArgs(i)
			}
		}

		content := f.String()
		// simulation genesis state
		templateGs := `%[1]vList: []types.%[1]v{
		{
			%[2]v},
		{
			%[3]v},
	}`
		genesisStateSnippet := fmt.Sprintf(
			templateGs,
			opts.TypeName.UpperCamel,
			sampleIndexes[0],
			sampleIndexes[1],
		)
		if strings.Count(content, typed.PlaceholderSimappGenesisState) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			genesisStateSnippet += ",\n" + typed.PlaceholderSimappGenesisState
			content = replacer.Replace(content, typed.PlaceholderSimappGenesisState, genesisStateSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
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
