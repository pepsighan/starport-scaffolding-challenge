package maptype

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/placeholder"
	"github.com/tendermint/starport/starport/pkg/xgenny"
	"github.com/tendermint/starport/starport/templates/field/datatype"
	"github.com/tendermint/starport/starport/templates/module"
	"github.com/tendermint/starport/starport/templates/typed"
)

var (
	//go:embed stargate/component/* stargate/component/**/*
	fsStargateComponent embed.FS

	//go:embed stargate/messages/* stargate/messages/**/*
	fsStargateMessages embed.FS

	//go:embed stargate/tests/component/* stargate/tests/component/**/*
	fsStargateTestsComponent embed.FS

	//go:embed stargate/tests/messages/* stargate/tests/messages/**/*
	fsStargateTestsMessages embed.FS
)

// NewStargate returns the generator to scaffold a new map type in a Stargate module
func NewStargate(replacer placeholder.Replacer, opts *typed.Options) (*genny.Generator, error) {
	// Tests are not generated for map with a custom index that contains only booleans
	// because we can't generate reliable tests for this type
	var generateTest bool
	for _, index := range opts.Indexes {
		if index.DatatypeName != datatype.Bool {
			generateTest = true
		}
	}

	var (
		g = genny.New()

		messagesTemplate = xgenny.NewEmbedWalker(
			fsStargateMessages,
			"stargate/messages/",
			opts.AppPath,
		)
		testsMessagesTemplate = xgenny.NewEmbedWalker(
			fsStargateTestsMessages,
			"stargate/tests/messages/",
			opts.AppPath,
		)
		componentTemplate = xgenny.NewEmbedWalker(
			fsStargateComponent,
			"stargate/component/",
			opts.AppPath,
		)
		testsComponentTemplate = xgenny.NewEmbedWalker(
			fsStargateTestsComponent,
			"stargate/tests/component/",
			opts.AppPath,
		)
	)

	g.RunFn(protoRPCModify(replacer, opts))
	g.RunFn(moduleGRPCGatewayModify(replacer, opts))
	g.RunFn(clientCliQueryModify(replacer, opts))
	g.RunFn(genesisProtoModify(opts))
	g.RunFn(genesisTypesModify(replacer, opts))
	g.RunFn(genesisModuleModify(replacer, opts))
	g.RunFn(genesisTestsModify(replacer, opts))
	g.RunFn(genesisTypesTestsModify(replacer, opts))

	// Modifications for new messages
	if !opts.NoMessage {
		g.RunFn(protoTxModify(replacer, opts))
		g.RunFn(handlerModify(replacer, opts))
		g.RunFn(clientCliTxModify(replacer, opts))
		g.RunFn(typesCodecModify(replacer, opts))
		g.RunFn(moduleSimulationModify(replacer, opts))

		if err := typed.Box(messagesTemplate, opts, g); err != nil {
			return nil, err
		}
		if generateTest {
			if err := typed.Box(testsMessagesTemplate, opts, g); err != nil {
				return nil, err
			}
		}
	}

	if generateTest {
		if err := typed.Box(testsComponentTemplate, opts, g); err != nil {
			return nil, err
		}
	}
	return g, typed.Box(componentTemplate, opts, g)
}

func protoRPCModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "query.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := strings.ReplaceAll(f.String(), `
import "gogoproto/gogo.proto";`, "")

		// Import the type
		templateImport := `
import "gogoproto/gogo.proto";
import "%s/%s.proto";`
		importString := fmt.Sprintf(templateImport,
			opts.ModuleName,
			opts.TypeName.Snake,
		)

		content, err = clipper.PasteProtoImportSnippetAt(path, content, importString)
		if err != nil {
			return err
		}

		var lowerCamelIndexes []string
		for _, index := range opts.Indexes {
			lowerCamelIndexes = append(lowerCamelIndexes, fmt.Sprintf("{%s}", index.Name.LowerCamel))
		}
		indexPath := strings.Join(lowerCamelIndexes, "/")

		// Add the service
		templateService := `
  // Queries a %[2]v by index.
	rpc %[1]v(QueryGet%[1]vRequest) returns (QueryGet%[1]vResponse) {
		option (google.api.http).get = "/%[3]v/%[4]v/%[5]v/%[2]v/%[6]v";
	}

	// Queries a list of %[2]v items.
	rpc %[1]vAll(QueryAll%[1]vRequest) returns (QueryAll%[1]vResponse) {
		option (google.api.http).get = "/%[3]v/%[4]v/%[5]v/%[2]v";
	}
`
		replacementService := fmt.Sprintf(templateService,
			opts.TypeName.UpperCamel,
			opts.TypeName.LowerCamel,
			opts.OwnerName,
			opts.AppName,
			opts.ModuleName,
			indexPath,
		)

		if strings.Count(content, typed.Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			replacementService += typed.Placeholder2
			content = replacer.Replace(content, typed.Placeholder2, replacementService)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteCodeSnippetAt(
				path,
				content,
				clipper.ProtoSelectNewServiceMethodPosition,
				clipper.SelectOptions{
					"name": "Query",
				},
				replacementService,
			)
			if err != nil {
				return err
			}
		}

		// Add the service messages
		var queryIndexFields string
		for i, index := range opts.Indexes {
			queryIndexFields += fmt.Sprintf("  %s;\n", index.ProtoType(i+1))
		}

		// Ensure custom types are imported
		protoImports := opts.Fields.ProtoImports()
		for _, f := range opts.Fields.Custom() {
			protoImports = append(protoImports,
				fmt.Sprintf("%[1]v/%[2]v.proto", opts.ModuleName, f),
			)
		}
		for _, f := range protoImports {
			importModule := fmt.Sprintf(`
import "%[1]v";`, f)
			content = strings.ReplaceAll(content, importModule, "")

			content, err = clipper.PasteProtoImportSnippetAt(path, content, importModule)
			if err != nil {
				return err
			}
		}

		templateMessage := `

message QueryGet%[1]vRequest {
	%[3]v
}

message QueryGet%[1]vResponse {
	%[1]v %[2]v = 1 [(gogoproto.nullable) = false];
}

message QueryAll%[1]vRequest {
	cosmos.base.query.v1beta1.PageRequest pagination = 1;
}

message QueryAll%[1]vResponse {
	repeated %[1]v %[2]v = 1 [(gogoproto.nullable) = false];
	cosmos.base.query.v1beta1.PageResponse pagination = 2;
}`
		replacementMessage := fmt.Sprintf(templateMessage,
			opts.TypeName.UpperCamel,
			opts.TypeName.LowerCamel,
			queryIndexFields,
		)
		content, err = clipper.PasteCodeSnippetAt(
			path,
			content,
			clipper.ProtoSelectLastPosition,
			nil,
			replacementMessage,
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func moduleGRPCGatewayModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "module.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}
		snippet := `"context"`
		content, err := clipper.PasteGoImportSnippetAt(path, f.String(), snippet)
		if err != nil {
			return err
		}

		snippet = `types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))`
		content = replacer.ReplaceOnce(content, typed.Placeholder2, snippet)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func clientCliQueryModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "client/cli/query.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		template := `cmd.AddCommand(CmdList%[1]v())
	cmd.AddCommand(CmdShow%[1]v())`
		snippet := fmt.Sprintf(template,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.Placeholder) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += "\n" + typed.Placeholder
			content = replacer.Replace(f.String(), typed.Placeholder, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, snippet, clipper.SelectOptions{
				"functionName": "GetQueryCmd",
			})
			if err != nil {
				return err
			}
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
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

		if strings.Count(content, typed.PlaceholderGenesisTypesDefault) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			funcArgSnippet += "\n" + typed.PlaceholderGenesisTypesDefault
			content = replacer.Replace(content, typed.PlaceholderGenesisTypesDefault, funcArgSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteGoReturningCompositeNewArgumentSnippetAt(
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

		// lines of code to call the key function with the indexes of the element
		var indexArgs []string
		for _, index := range opts.Indexes {
			indexArgs = append(indexArgs, "elem."+index.Name.UpperCamel)
		}
		keyCall := fmt.Sprintf("%sKey(%s)", opts.TypeName.UpperCamel, strings.Join(indexArgs, ","))

		templateTypesValidate := `// Check for duplicated index in %[1]v
	%[1]vIndexMap := make(map[string]struct{})
	
	for _, elem := range gs.%[2]vList {
		index := %[3]v
		if _, ok := %[1]vIndexMap[index]; ok {
			return fmt.Errorf("duplicated index for %[1]v")
		}
		%[1]vIndexMap[index] = struct{}{}
	}`
		beforeReturnSnippet := fmt.Sprintf(
			templateTypesValidate,
			opts.TypeName.LowerCamel,
			opts.TypeName.UpperCamel,
			fmt.Sprintf("string(%s)", keyCall),
		)

		if strings.Count(content, typed.PlaceholderGenesisTypesValidate) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			beforeReturnSnippet += "\n" + typed.PlaceholderGenesisTypesValidate
			content = replacer.Replace(content, typed.PlaceholderGenesisTypesValidate, beforeReturnSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
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

func genesisModuleModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
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
  }`
		moduleInitSnippet := fmt.Sprintf(
			templateModuleInit,
			opts.TypeName.LowerCamel,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.PlaceholderGenesisModuleInit) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			moduleInitSnippet += "\n" + typed.PlaceholderGenesisModuleInit
			content = replacer.Replace(content, typed.PlaceholderGenesisModuleInit, moduleInitSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteCodeSnippetAt(
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

		templateModuleExport := `genesis.%[1]vList = k.GetAll%[1]v(ctx)`
		moduleExport := fmt.Sprintf(
			templateModuleExport,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.PlaceholderGenesisModuleExport) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			moduleExport += "\n" + typed.PlaceholderGenesisModuleExport
			content = replacer.Replace(content, typed.PlaceholderGenesisModuleExport, moduleExport)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, moduleExport, clipper.SelectOptions{
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

func genesisTestsModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "genesis_test.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Create a list of two different indexes to use as sample
		sampleIndexes := make([]string, 2)
		for i := 0; i < 2; i++ {
			for _, index := range opts.Indexes {
				sampleIndexes[i] += index.GenesisArgs(i)
			}
		}

		content := f.String()

		templateState := `%[1]vList: []types.%[1]v{
		{
			%[2]v},
		{
			%[3]v},
	}`
		testStateSnippet := fmt.Sprintf(
			templateState,
			opts.TypeName.UpperCamel,
			sampleIndexes[0],
			sampleIndexes[1],
		)

		if strings.Count(content, module.PlaceholderGenesisTestState) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			testStateSnippet += ",\n" + module.PlaceholderGenesisTestState
			content = replacer.Replace(content, module.PlaceholderGenesisTestState, testStateSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteGoReturningCompositeNewArgumentSnippetAt(
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

		templateAssert := `require.ElementsMatch(t, genesisState.%[1]vList, got.%[1]vList)`
		beforeReturnSnippet := fmt.Sprintf(
			templateAssert,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, module.PlaceholderGenesisTestAssert) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			beforeReturnSnippet += "\n" + module.PlaceholderGenesisTestAssert
			content = replacer.Replace(content, module.PlaceholderGenesisTestAssert, beforeReturnSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
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

func genesisTypesTestsModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/genesis_test.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Create a list of two different indexes to use as sample
		sampleIndexes := make([]string, 2)
		for i := 0; i < 2; i++ {
			for _, index := range opts.Indexes {
				sampleIndexes[i] += index.GenesisArgs(i)
			}
		}

		content := f.String()
		templateValid := `%[1]vList: []types.%[1]v{
	{
		%[2]v},
	{
		%[3]v},
}`
		validFieldSnippet := fmt.Sprintf(
			templateValid,
			opts.TypeName.UpperCamel,
			sampleIndexes[0],
			sampleIndexes[1],
		)

		if strings.Count(content, module.PlaceholderTypesGenesisValidField) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			validFieldSnippet += ",\n" + module.PlaceholderTypesGenesisValidField
			content = replacer.Replace(content, module.PlaceholderTypesGenesisValidField, validFieldSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
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

		templateDuplicated := `{
	desc:     "duplicated %[2]v",
	genState: &types.GenesisState{
		%[3]vList: []types.%[3]v{
			{
				%[4]v},
			{
				%[4]v},
		},
	},
	valid:    false,
},
%[1]v`
		replacementDuplicated := fmt.Sprintf(
			templateDuplicated,
			module.PlaceholderTypesGenesisTestcase,
			opts.TypeName.LowerCamel,
			opts.TypeName.UpperCamel,
			sampleIndexes[0],
		)
		content = replacer.Replace(content, module.PlaceholderTypesGenesisTestcase, replacementDuplicated)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func protoTxModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "tx.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Import
		templateImport := `
import "%s/%s.proto";`
		importString := fmt.Sprintf(templateImport,
			opts.ModuleName,
			opts.TypeName.Snake,
		)

		content, err := clipper.PasteProtoImportSnippetAt(path, f.String(), importString)
		if err != nil {
			return err
		}

		// RPC service
		templateRPC := `  rpc Create%[1]v(MsgCreate%[1]v) returns (MsgCreate%[1]vResponse);
  rpc Update%[1]v(MsgUpdate%[1]v) returns (MsgUpdate%[1]vResponse);
  rpc Delete%[1]v(MsgDelete%[1]v) returns (MsgDelete%[1]vResponse);
`
		serviceSnippet := fmt.Sprintf(templateRPC, opts.TypeName.UpperCamel)

		if strings.Count(content, typed.PlaceholderProtoTxRPC) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			serviceSnippet += typed.PlaceholderProtoTxRPC
			content = replacer.Replace(content, typed.PlaceholderProtoTxRPC, serviceSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteCodeSnippetAt(
				path,
				content,
				clipper.ProtoSelectNewServiceMethodPosition,
				clipper.SelectOptions{
					"name": "Msg",
				},
				serviceSnippet,
			)
			if err != nil {
				return err
			}
		}

		// Messages
		var indexes string
		for i, index := range opts.Indexes {
			indexes += fmt.Sprintf("  %s;\n", index.ProtoType(i+2))
		}

		var fields string
		for i, f := range opts.Fields {
			fields += fmt.Sprintf("  %s;\n", f.ProtoType(i+2+len(opts.Indexes)))
		}

		// Ensure custom types are imported
		protoImports := append(opts.Fields.ProtoImports(), opts.Indexes.ProtoImports()...)
		customFields := append(opts.Fields.Custom(), opts.Indexes.Custom()...)
		for _, f := range customFields {
			protoImports = append(protoImports,
				fmt.Sprintf("%[1]v/%[2]v.proto", opts.ModuleName, f),
			)
		}
		for _, f := range protoImports {
			importModule := fmt.Sprintf(`
import "%[1]v";`, f)
			content = strings.ReplaceAll(content, importModule, "")

			content, err = clipper.PasteProtoImportSnippetAt(path, content, importModule)
			if err != nil {
				return err
			}
		}

		templateMessages := `

message MsgCreate%[1]v {
  string %[2]v = 1;
%[3]v
%[4]v}
message MsgCreate%[1]vResponse {}

message MsgUpdate%[1]v {
  string %[2]v = 1;
%[3]v
%[4]v}
message MsgUpdate%[1]vResponse {}

message MsgDelete%[1]v {
  string %[2]v = 1;
%[3]v}
message MsgDelete%[1]vResponse {}`
		replacementMessages := fmt.Sprintf(templateMessages,
			opts.TypeName.UpperCamel,
			opts.MsgSigner.LowerCamel,
			indexes,
			fields,
		)
		content, err = clipper.PasteCodeSnippetAt(
			path,
			content,
			clipper.ProtoSelectLastPosition,
			nil,
			replacementMessages,
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func handlerModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "handler.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Set once the MsgServer definition if it is not defined yet
		replacementMsgServer := `msgServer := keeper.NewMsgServerImpl(k)`
		content := replacer.ReplaceOnce(f.String(), typed.PlaceholderHandlerMsgServer, replacementMsgServer)

		templateHandlers := `case *types.MsgCreate%[2]v:
					res, err := msgServer.Create%[2]v(sdk.WrapSDKContext(ctx), msg)
					return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgUpdate%[2]v:
					res, err := msgServer.Update%[2]v(sdk.WrapSDKContext(ctx), msg)
					return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgDelete%[2]v:
					res, err := msgServer.Delete%[2]v(sdk.WrapSDKContext(ctx), msg)
					return sdk.WrapServiceResult(ctx, res, err)
%[1]v`
		replacementHandlers := fmt.Sprintf(templateHandlers,
			typed.Placeholder,
			opts.TypeName.UpperCamel,
		)
		content = replacer.Replace(content, typed.Placeholder, replacementHandlers)
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func clientCliTxModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "client/cli/tx.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		template := `cmd.AddCommand(CmdCreate%[1]v())
	cmd.AddCommand(CmdUpdate%[1]v())
	cmd.AddCommand(CmdDelete%[1]v())`
		snippet := fmt.Sprintf(template, opts.TypeName.UpperCamel)

		if strings.Count(content, typed.Placeholder) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += "\n" + typed.Placeholder
			content = replacer.Replace(content, typed.Placeholder, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, snippet, clipper.SelectOptions{
				"functionName": "GetTxCmd",
			})
			if err != nil {
				return err
			}
		}
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func typesCodecModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/codec.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		// Import
		importSnippet := `sdk "github.com/cosmos/cosmos-sdk/types"`
		content, err = clipper.PasteGoImportSnippetAt(path, content, importSnippet)
		if err != nil {
			return err
		}

		// Concrete
		templateConcrete := `
	cdc.RegisterConcrete(&MsgCreate%[1]v{}, "%[2]v/Create%[1]v", nil)
	cdc.RegisterConcrete(&MsgUpdate%[1]v{}, "%[2]v/Update%[1]v", nil)
	cdc.RegisterConcrete(&MsgDelete%[1]v{}, "%[2]v/Delete%[1]v", nil)`
		startOfFunctionSnippet := fmt.Sprintf(
			templateConcrete,
			opts.TypeName.UpperCamel,
			opts.ModuleName,
		)

		if strings.Count(content, typed.Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + typed.Placeholder2
			content = replacer.Replace(content, typed.Placeholder2, startOfFunctionSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteCodeSnippetAt(
				path,
				content,
				clipper.GoSelectStartOfFunctionPosition,
				clipper.SelectOptions{
					"functionName": "RegisterCodec",
				},
				startOfFunctionSnippet,
			)
			if err != nil {
				return err
			}
		}

		// Interface
		templateInterface := `
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreate%[1]v{},
		&MsgUpdate%[1]v{},
		&MsgDelete%[1]v{},
	)`
		startOfFunctionSnippet = fmt.Sprintf(
			templateInterface,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.Placeholder3) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + typed.Placeholder3
			content = replacer.Replace(content, typed.Placeholder3, startOfFunctionSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteCodeSnippetAt(
				path,
				content,
				clipper.GoSelectStartOfFunctionPosition,
				clipper.SelectOptions{
					"functionName": "RegisterInterfaces",
				},
				startOfFunctionSnippet,
			)
			if err != nil {
				return err
			}
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
