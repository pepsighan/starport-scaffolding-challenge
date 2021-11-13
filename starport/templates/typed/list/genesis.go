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
	g.RunFn(genesisProtoModify(replacer, opts))
	g.RunFn(genesisTypesModify(replacer, opts))
	g.RunFn(genesisModuleModify(replacer, opts))
	g.RunFn(genesisTestsModify(replacer, opts))
	g.RunFn(genesisTypesTestsModify(replacer, opts))
}

func genesisProtoModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
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
		replacementProtoImport := fmt.Sprintf(
			templateProtoImport,
			opts.ModuleName,
			opts.TypeName.Snake,
		)
		content, err = clipper.PasteProtoSnippetAt(
			content,
			clipper.ProtoSelectNewImportPosition,
			nil,
			replacementProtoImport,
		)
		if err != nil {
			return err
		}

		templateProtoState := `
  repeated %[1]v %[2]vList = %[3]v [(gogoproto.nullable) = false];
  uint64 %[2]vCount = %[4]v;
`
		content, err = clipper.PasteGeneratedProtoSnippetAt(
			content,
			clipper.ProtoSelectNewMessageFieldPosition,
			clipper.SelectOptions{
				"name": "GenesisState",
			},
			func(data map[string]interface{}) string {
				highestNumber := data["highestFieldNumber"].(int)

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

		content := typed.PatchGenesisTypeImport(replacer, f.String())

		templateTypesImport := `"fmt"`
		content = replacer.ReplaceOnce(content, typed.PlaceholderGenesisTypesImport, templateTypesImport)

		templateTypesDefault := `%[2]vList: []%[2]v{},
%[1]v`
		replacementTypesDefault := fmt.Sprintf(
			templateTypesDefault,
			typed.PlaceholderGenesisTypesDefault,
			opts.TypeName.UpperCamel,
		)
		content = replacer.Replace(content, typed.PlaceholderGenesisTypesDefault, replacementTypesDefault)

		templateTypesValidate := `// Check for duplicated ID in %[2]v
%[2]vIdMap := make(map[uint64]bool)
%[2]vCount := gs.Get%[3]vCount()
for _, elem := range gs.%[3]vList {
	if _, ok := %[2]vIdMap[elem.Id]; ok {
		return fmt.Errorf("duplicated id for %[2]v")
	}
	if elem.Id >= %[2]vCount {
		return fmt.Errorf("%[2]v id should be lower or equal than the last id")
	}
	%[2]vIdMap[elem.Id] = true
}
%[1]v`
		replacementTypesValidate := fmt.Sprintf(
			templateTypesValidate,
			typed.PlaceholderGenesisTypesValidate,
			opts.TypeName.LowerCamel,
			opts.TypeName.UpperCamel,
		)
		content = replacer.Replace(content, typed.PlaceholderGenesisTypesValidate, replacementTypesValidate)

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

		templateModuleInit := `// Set all the %[2]v
for _, elem := range genState.%[3]vList {
	k.Set%[3]v(ctx, elem)
}

// Set %[2]v count
k.Set%[3]vCount(ctx, genState.%[3]vCount)
%[1]v`
		replacementModuleInit := fmt.Sprintf(
			templateModuleInit,
			typed.PlaceholderGenesisModuleInit,
			opts.TypeName.LowerCamel,
			opts.TypeName.UpperCamel,
		)
		content := replacer.Replace(f.String(), typed.PlaceholderGenesisModuleInit, replacementModuleInit)

		templateModuleExport := `genesis.%[2]vList = k.GetAll%[2]v(ctx)
genesis.%[2]vCount = k.Get%[2]vCount(ctx)
%[1]v`
		replacementModuleExport := fmt.Sprintf(
			templateModuleExport,
			typed.PlaceholderGenesisModuleExport,
			opts.TypeName.UpperCamel,
		)
		content = replacer.Replace(content, typed.PlaceholderGenesisModuleExport, replacementModuleExport)

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

		templateAssert := `require.ElementsMatch(t, genesisState.%[2]vList, got.%[2]vList)
require.Equal(t, genesisState.%[2]vCount, got.%[2]vCount)
%[1]v`
		replacementTests := fmt.Sprintf(
			templateAssert,
			module.PlaceholderGenesisTestAssert,
			opts.TypeName.UpperCamel,
		)
		content = replacer.Replace(content, module.PlaceholderGenesisTestAssert, replacementTests)

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

		templateValid := `%[2]vList: []types.%[2]v{
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
			templateValid,
			module.PlaceholderTypesGenesisValidField,
			opts.TypeName.UpperCamel,
		)
		content := replacer.Replace(f.String(), module.PlaceholderTypesGenesisValidField, replacementValid)

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
