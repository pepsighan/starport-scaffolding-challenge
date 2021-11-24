package singleton

import (
	"embed"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/xgenny"
	"github.com/tendermint/starport/starport/templates/module"
	"github.com/tendermint/starport/starport/templates/typed"
)

var (
	//go:embed stargate/component/* stargate/component/**/*
	fsStargateComponent embed.FS

	//go:embed stargate/messages/* stargate/messages/**/*
	fsStargateMessages embed.FS
)

// NewStargate returns the generator to scaffold a new indexed type in a Stargate module
func NewStargate(clip *clipper.Clipper, opts *typed.Options) (*genny.Generator, error) {
	var (
		g = genny.New()

		messagesTemplate = xgenny.NewEmbedWalker(
			fsStargateMessages,
			"stargate/messages/",
			opts.AppPath,
		)
		componentTemplate = xgenny.NewEmbedWalker(
			fsStargateComponent,
			"stargate/component/",
			opts.AppPath,
		)
	)

	g.RunFn(typesKeyModify(opts))
	g.RunFn(protoRPCModify(clip, opts))
	g.RunFn(moduleGRPCGatewayModify(clip, opts))
	g.RunFn(clientCliQueryModify(clip, opts))
	g.RunFn(genesisProtoModify(clip, opts))
	g.RunFn(genesisTypesModify(clip, opts))
	g.RunFn(genesisModuleModify(clip, opts))
	g.RunFn(genesisTestsModify(clip, opts))
	g.RunFn(genesisTypesTestsModify(clip, opts))

	// Modifications for new messages
	if !opts.NoMessage {
		g.RunFn(protoTxModify(clip, opts))
		g.RunFn(handlerModify(clip, opts))
		g.RunFn(clientCliTxModify(clip, opts))
		g.RunFn(typesCodecModify(clip, opts))
		g.RunFn(moduleSimulationModify(clip, opts))

		if err := typed.Box(messagesTemplate, opts, g); err != nil {
			return nil, err
		}
	}

	return g, typed.Box(componentTemplate, opts, g)
}

func typesKeyModify(opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/keys.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}
		content := f.String() + fmt.Sprintf(`
const (
	%[1]vKey= "%[1]v-value-"
)
`, opts.TypeName.UpperCamel)
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func protoRPCModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
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

		content, err = clip.PasteProtoImportSnippetAt(path, content, importString)
		if err != nil {
			return err
		}

		// Add the service
		templateService := `
  // Queries a %[2]v by index.
	rpc %[1]v(QueryGet%[1]vRequest) returns (QueryGet%[1]vResponse) {
		option (google.api.http).get = "/%[3]v/%[4]v/%[5]v/%[2]v";
	}
`
		serviceSnippet := fmt.Sprintf(templateService,
			opts.TypeName.UpperCamel,
			opts.TypeName.LowerCamel,
			opts.OwnerName,
			opts.AppName,
			opts.ModuleName,
		)

		if strings.Count(content, typed.Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			serviceSnippet += typed.Placeholder2
			content = clip.Replace(content, typed.Placeholder2, serviceSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.ProtoSelectNewServiceMethodPosition,
				clipper.SelectOptions{
					"name": "Query",
				},
				serviceSnippet,
			)
			if err != nil {
				return err
			}
		}

		// Add the service messages
		templateMessage := `

message QueryGet%[1]vRequest {}

message QueryGet%[1]vResponse {
	%[1]v %[1]v = 1 [(gogoproto.nullable) = false];
}`
		replacementMessage := fmt.Sprintf(templateMessage, opts.TypeName.UpperCamel)
		content, err = clip.PasteCodeSnippetAt(
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

func moduleGRPCGatewayModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "module.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}
		snippet := `"context"`
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), snippet)
		if err != nil {
			return err
		}

		snippet = `types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))`
		content = clip.ReplaceOnce(content, typed.Placeholder2, snippet)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func clientCliQueryModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "client/cli/query.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		template := `cmd.AddCommand(CmdShow%[1]v())`
		snippet := fmt.Sprintf(template,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.Placeholder) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += "\n" + typed.Placeholder
			content = clip.Replace(content, typed.Placeholder, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, snippet, clipper.SelectOptions{
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

func genesisProtoModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "genesis.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		templateProtoImport := `
import "%[1]v/%[2]v.proto";`
		importString := fmt.Sprintf(
			templateProtoImport,
			opts.ModuleName,
			opts.TypeName.Snake,
		)

		content, err := clip.PasteProtoImportSnippetAt(path, f.String(), importString)
		if err != nil {
			return err
		}

		templateProtoState := `  %[1]v %[2]v = %[3]v;
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

		content := f.String()

		templateTypesDefault := `%[1]v: nil`
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

		// Create a fields
		sampleFields := ""
		for _, field := range opts.Fields {
			sampleFields += field.GenesisArgs(rand.Intn(100) + 1)
		}

		content := f.String()

		templateState := `%[2]v: &types.%[2]v{
		%[3]v},
		%[1]v`
		testStateSnippet := fmt.Sprintf(
			templateState,
			module.PlaceholderGenesisTestState,
			opts.TypeName.UpperCamel,
			sampleFields,
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

		templateAssert := `require.Equal(t, genesisState.%[1]v, got.%[1]v)`
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

		// Create a fields
		sampleFields := ""
		for _, field := range opts.Fields {
			sampleFields += field.GenesisArgs(rand.Intn(100) + 1)
		}

		content := f.String()
		templateValid := `%[1]v: &types.%[1]v{
		%[2]v}`
		validFieldSnippet := fmt.Sprintf(
			templateValid,
			opts.TypeName.UpperCamel,
			sampleFields,
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
	// Set if defined
	if genState.%[1]v != nil {
		k.Set%[1]v(ctx, *genState.%[1]v)
	}`
		moduleInitSnippet := fmt.Sprintf(
			templateModuleInit,
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

		templateModuleExport := `// Get all %[1]v
  %[1]v, found := k.Get%[2]v(ctx)
  if found {
	  genesis.%[2]v = &%[1]v
  }`
		moduleExport := fmt.Sprintf(
			templateModuleExport,
			opts.TypeName.LowerCamel,
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

func protoTxModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
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

		content, err := clip.PasteProtoImportSnippetAt(path, f.String(), importString)
		if err != nil {
			return err
		}

		// RPC service
		templateRPC := `  rpc Create%[1]v(MsgCreate%[1]v) returns (MsgCreate%[1]vResponse);
  rpc Update%[1]v(MsgUpdate%[1]v) returns (MsgUpdate%[1]vResponse);
  rpc Delete%[1]v(MsgDelete%[1]v) returns (MsgDelete%[1]vResponse);
`
		serviceSnippet := fmt.Sprintf(templateRPC,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.PlaceholderProtoTxRPC) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			serviceSnippet += typed.PlaceholderProtoTxRPC
			content = clip.Replace(content, typed.PlaceholderProtoTxRPC, serviceSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
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
		var fields string
		for i, field := range opts.Fields {
			fields += fmt.Sprintf("  %s;\n", field.ProtoType(i+3))
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

			content, err = clip.PasteProtoImportSnippetAt(path, content, importModule)
			if err != nil {
				return err
			}
		}

		templateMessages := `

message MsgCreate%[1]v {
  string %[2]v = 1;
%[3]v}
message MsgCreate%[1]vResponse {}

message MsgUpdate%[1]v {
  string %[2]v = 1;
%[3]v}
message MsgUpdate%[1]vResponse {}

message MsgDelete%[1]v {
  string %[2]v = 1;
}
message MsgDelete%[1]vResponse {}`
		replacementMessages := fmt.Sprintf(templateMessages,
			opts.TypeName.UpperCamel,
			opts.MsgSigner.LowerCamel,
			fields,
		)
		content, err = clip.PasteCodeSnippetAt(
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

func handlerModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "handler.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Set once the MsgServer definition if it is not defined yet
		replacementMsgServer := `msgServer := keeper.NewMsgServerImpl(k)`
		content := clip.ReplaceOnce(f.String(), typed.PlaceholderHandlerMsgServer, replacementMsgServer)

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
		content = clip.Replace(content, typed.Placeholder, replacementHandlers)
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func clientCliTxModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
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
			content = clip.Replace(content, typed.Placeholder, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, f.String(), snippet, clipper.SelectOptions{
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

func typesCodecModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/codec.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		// Import
		importSnippet := `sdk "github.com/cosmos/cosmos-sdk/types"`
		content, err = clip.PasteGoImportSnippetAt(path, content, importSnippet)
		if err != nil {
			return err
		}

		// Concrete
		templateConcrete := `
  cdc.RegisterConcrete(&MsgCreate%[1]v{}, "%[2]v/Create%[1]v", nil)
	cdc.RegisterConcrete(&MsgUpdate%[1]v{}, "%[2]v/Update%[1]v", nil)
	cdc.RegisterConcrete(&MsgDelete%[1]v{}, "%[2]v/Delete%[1]v", nil)`
		functionStartSnippet := fmt.Sprintf(
			templateConcrete,
			opts.TypeName.UpperCamel,
			opts.ModuleName,
		)

		if strings.Count(content, typed.Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			functionStartSnippet += "\n" + typed.Placeholder2
			content = clip.Replace(content, typed.Placeholder2, functionStartSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.GoSelectStartOfFunctionPosition,
				clipper.SelectOptions{
					"functionName": "RegisterCodec",
				},
				functionStartSnippet,
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
		functionStartSnippet = fmt.Sprintf(
			templateInterface,
			opts.TypeName.UpperCamel,
		)

		if strings.Count(content, typed.Placeholder3) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			functionStartSnippet += "\n" + typed.Placeholder3
			content = clip.Replace(content, typed.Placeholder3, functionStartSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.GoSelectStartOfFunctionPosition,
				clipper.SelectOptions{
					"functionName": "RegisterInterfaces",
				},
				functionStartSnippet,
			)
			if err != nil {
				return err
			}
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
