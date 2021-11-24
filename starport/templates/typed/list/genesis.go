package list

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/templates/module"
	"github.com/tendermint/starport/starport/templates/typed"
)

func genesisModify(clip *clipper.Clipper, opts *typed.Options, g *genny.Generator) {
	g.RunFn(genesisProtoModify(clip, opts))
	g.RunFn(genesisTypesModify(clip, opts))
	g.RunFn(genesisModuleModify(clip, opts))
	g.RunFn(genesisTestsModify(clip, opts))
	g.RunFn(genesisTypesTestsModify(clip, opts))
}

func genesisProtoModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
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

		content, err = clip.PasteProtoImportSnippetAt(path, content, importString)
		if err != nil {
			return err
		}

		templateProtoState := `  repeated %[1]v %[2]vList = %[3]v [(gogoproto.nullable) = false];
  uint64 %[2]vCount = %[4]v;
`
		content, err = clip.PasteGeneratedCodeSnippetAt(
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

func genesisTypesModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/genesis.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// There can be duplicate imports of `"fmt"` when the command is run more
		// times but the gofmt will remove the duplicate ones.
		importSnippet := `"fmt"`
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		templateTypesDefault := `%[1]vList: []%[1]v{}`
		funcArgSnippet := fmt.Sprintf(
			templateTypesDefault,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.PlaceholderGenesisTypesDefault) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			funcArgSnippet += "\n" + typed.PlaceholderGenesisTypesDefault
			content = clip.Replace(content, typed.PlaceholderGenesisTypesDefault, funcArgSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningCompositeNewArgumentSnippetAt(
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
		}

		templateTypesValidate := `// Check for duplicated id in %[1]v
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

		if strings.Count(content, typed.PlaceholderGenesisTypesValidate) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			beforeReturnSnippet += "\n" + typed.PlaceholderGenesisTypesValidate
			content = clip.Replace(content, typed.PlaceholderGenesisTypesValidate, beforeReturnSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
				"functionName": "Validate",
			})
			if err != nil {
				return err
			}
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func genesisModuleModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "genesis.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

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

		if strings.Count(content, typed.PlaceholderGenesisModuleInit) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			moduleInitSnippet += "\n" + typed.PlaceholderGenesisModuleInit
			content = clip.Replace(content, typed.PlaceholderGenesisModuleInit, moduleInitSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.GoSelectStartOfFunctionPosition,
				clipper.SelectOptions{
					"functionName": "InitGenesis",
				},
				moduleInitSnippet,
			)
			if err != nil {
				return err
			}
		}

		templateModuleExport := `genesis.%[1]vList = k.GetAll%[1]v(ctx)
  genesis.%[1]vCount = k.Get%[1]vCount(ctx)`
		moduleExport := fmt.Sprintf(
			templateModuleExport,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.PlaceholderGenesisModuleExport) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			moduleExport += "\n" + typed.PlaceholderGenesisModuleExport
			content = clip.Replace(content, typed.PlaceholderGenesisModuleExport, moduleExport)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, moduleExport, clipper.SelectOptions{
				"functionName": "ExportGenesis",
			})
			if err != nil {
				return err
			}
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func genesisTestsModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "genesis_test.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		templateState := `%[1]vList: []types.%[1]v{
		{
			Id: 0,
		},
		{
			Id: 1,
		},
	},
	%[1]vCount: 2`
		testStateSnippet := fmt.Sprintf(
			templateState,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, module.PlaceholderGenesisTestState) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			testStateSnippet += ",\n" + module.PlaceholderGenesisTestState
			content = clip.Replace(content, module.PlaceholderGenesisTestState, testStateSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningCompositeNewArgumentSnippetAt(
				path,
				content,
				testStateSnippet,
				clipper.SelectOptions{
					"functionName": "newTestGenesisState",
				},
			)
			if err != nil {
				return err
			}
		}

		templateAssert := `require.ElementsMatch(t, genesisState.%[1]vList, got.%[1]vList)
  require.Equal(t, genesisState.%[1]vCount, got.%[1]vCount)`
		beforeReturnSnippet := fmt.Sprintf(
			templateAssert,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, module.PlaceholderGenesisTestAssert) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			beforeReturnSnippet += "\n" + module.PlaceholderGenesisTestAssert
			content = clip.Replace(content, module.PlaceholderGenesisTestAssert, beforeReturnSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
				"functionName": "TestGenesis",
			})
			if err != nil {
				return err
			}
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func genesisTypesTestsModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
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
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			validFieldSnippet += ",\n" + module.PlaceholderTypesGenesisValidField
			content = clip.Replace(content, module.PlaceholderTypesGenesisValidField, validFieldSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningCompositeNewArgumentSnippetAt(
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
		content = clip.Replace(content, module.PlaceholderTypesGenesisTestcase, replacementTests)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
