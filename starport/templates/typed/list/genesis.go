package list

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/placeholder"
	"github.com/tendermint/starport/starport/templates/module"
	"github.com/tendermint/starport/starport/templates/typed"
)

func genesisModify(replacer placeholder.Replacer, opts *typed.Options, g *genny.Generator) {
	g.RunFn(genesisProtoModify(opts))
	g.RunFn(genesisTypesModify(replacer, opts))
	g.RunFn(genesisModuleModify(replacer, opts))
	g.RunFn(genesisTestsModify(replacer, opts))
	g.RunFn(genesisTypesTestsModify(replacer, opts))
}

func genesisProtoModify(opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "genesis.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := strings.ReplaceAll(f.String(), `
import "gogoproto/gogo.proto";`, "")

		templateProtoImport := `
import "gogoproto/gogo.proto";
import "%[1]v/%[2]v.proto";`
		importString := fmt.Sprintf(
			templateProtoImport,
			opts.ModuleName,
			opts.TypeName.Snake,
		)

		content, err = clipper.PasteProtoImportSnippetAt(path, content, importString)
		if err != nil {
			return err
		}

		templateProtoState := `  repeated %[1]v %[2]vList = %[3]v [(gogoproto.nullable) = false];
  uint64 %[2]vCount = %[4]v;
`
		content, err = clipper.PasteGeneratedCodeSnippetAt(
			path,
			content,
			clipper.ProtoSelectNewMessageFieldPosition,
			clipper.SelectOptions{
				"name": "GenesisState",
			},
			func(data interface{}) string {
				highestNumber := data.(clipper.ProtoNewMessageFieldPositionData).HighestFieldNumber
				return fmt.Sprintf(
					templateProtoState,
					opts.TypeName.UpperCamel,
					opts.TypeName.LowerCamel,
					highestNumber+1,
					highestNumber+2,
				)
			},
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func genesisTypesModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/genesis.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// There can be duplicate imports of `"fmt"` when the command is run more
		// times but the gofmt will remove the duplicate ones.
		importSnippet := `"fmt"`
		content, err := clipper.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		templateTypesDefault := `%[1]vList: []%[1]v{}`
		funcArgSnippet := fmt.Sprintf(
			templateTypesDefault,
			opts.TypeName.UpperCamel,
		)
		content, err = clipper.PasteGoReturningFunctionNewArgumentSnippetAt(
			path,
			content,
			funcArgSnippet,
			clipper.SelectOptions{
				"functionName": "DefaultGenesis",
			},
		)
		if err != nil {
			return err
		}

		templateTypesValidate := `// Check for duplicated ID in %[1]v
	%[1]vIdMap := make(map[uint64]bool)
	%[1]vCount := gs.Get%[2]vCount()
	for _, elem := range gs.%[2]vList {
		if _, ok := %[1]vIdMap[elem.Id]; ok {
			return fmt.Errorf("duplicated id for %[1]v")
		}
		if elem.Id >= %[1]vCount {
			return fmt.Errorf("%[1]v id should be lower or equal than the last id")
		}
		%[1]vIdMap[elem.Id] = true
	}`
		beforeReturnSnippet := fmt.Sprintf(
			templateTypesValidate,
			opts.TypeName.LowerCamel,
			opts.TypeName.UpperCamel,
		)
		content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
			"functionName": "Validate",
		})
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func genesisModuleModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "genesis.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		templateModuleInit := `
	// Set all the %[1]v
	for _, elem := range genState.%[2]vList {
		k.Set%[2]v(ctx, elem)
	}
	
	// Set %[1]v count
	k.Set%[2]vCount(ctx, genState.%[2]vCount)`
		moduleInitSnippet := fmt.Sprintf(
			templateModuleInit,
			opts.TypeName.LowerCamel,
			opts.TypeName.UpperCamel,
		)
		content, err := clipper.PasteCodeSnippetAt(
			path,
			f.String(),
			clipper.GoSelectStartOfFunctionPosition,
			clipper.SelectOptions{
				"functionName": "InitGenesis",
			},
			moduleInitSnippet,
		)
		if err != nil {
			return err
		}

		templateModuleExport := `genesis.%[1]vList = k.GetAll%[1]v(ctx)
  genesis.%[1]vCount = k.Get%[1]vCount(ctx)`
		moduleExport := fmt.Sprintf(
			templateModuleExport,
			opts.TypeName.UpperCamel,
		)
		content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, moduleExport, clipper.SelectOptions{
			"functionName": "ExportGenesis",
		})
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func genesisTestsModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "genesis_test.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		templateState := `%[2]vList: []types.%[2]v{
		{
			Id: 0,
		},
		{
			Id: 1,
		},
	},
	%[2]vCount: 2,
	%[1]v`
		replacementValid := fmt.Sprintf(
			templateState,
			module.PlaceholderGenesisTestState,
			opts.TypeName.UpperCamel,
		)
		content := replacer.Replace(f.String(), module.PlaceholderGenesisTestState, replacementValid)

		templateAssert := `require.ElementsMatch(t, genesisState.%[1]vList, got.%[1]vList)
  require.Equal(t, genesisState.%[1]vCount, got.%[1]vCount)`
		beforeReturnSnippet := fmt.Sprintf(
			templateAssert,
			opts.TypeName.UpperCamel,
		)
		content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
			"functionName": "TestGenesis",
		})
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func genesisTypesTestsModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/genesis_test.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()
		templateValid := `%[1]vList: []types.%[1]v{
	{
		Id: 0,
	},
	{
		Id: 1,
	},
},
%[1]vCount: 2`
		validFieldSnippet := fmt.Sprintf(
			templateValid,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, module.PlaceholderTypesGenesisValidField) != 0 {
			// Use the older placeholder mechanism for older codebase.
			validFieldSnippet += ",\n" + module.PlaceholderTypesGenesisValidField
			content = replacer.Replace(content, module.PlaceholderTypesGenesisValidField, validFieldSnippet)
		} else {
			// Use the clipper based code generation for newer codebase.
			content, err = clipper.PasteGoReturningCompositeNewArgumentSnippetAt(
				path,
				content,
				validFieldSnippet,
				clipper.SelectOptions{
					"functionName": "newTestGenesisState",
				},
			)
			if err != nil {
				return err
			}
		}

		templateTests := `{
	desc:     "duplicated %[2]v",
	genState: &types.GenesisState{
		%[3]vList: []types.%[3]v{
			{
				Id: 0,
			},
			{
				Id: 0,
			},
		},
	},
	valid:    false,
},
{
	desc:     "invalid %[2]v count",
	genState: &types.GenesisState{
		%[3]vList: []types.%[3]v{
			{
				Id: 1,
			},
		},
		%[3]vCount: 0,
	},
	valid:    false,
},
%[1]v`
		replacementTests := fmt.Sprintf(
			templateTests,
			module.PlaceholderTypesGenesisTestcase,
			opts.TypeName.LowerCamel,
			opts.TypeName.UpperCamel,
		)
		content = replacer.Replace(content, module.PlaceholderTypesGenesisTestcase, replacementTests)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
