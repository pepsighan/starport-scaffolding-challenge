package list

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/xgenny"
	"github.com/tendermint/starport/starport/templates/typed"
)

var (
	//go:embed stargate/component/* stargate/component/**/*
	fsStargateComponent embed.FS

	//go:embed stargate/messages/* stargate/messages/**/*
	fsStargateMessages embed.FS
)

// NewStargate returns the generator to scaffold a new type in a Stargate module
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

	g.RunFn(protoQueryModify(clip, opts))
	g.RunFn(moduleGRPCGatewayModify(clip, opts))
	g.RunFn(typesKeyModify(opts))
	g.RunFn(clientCliQueryModify(clip, opts))

	// Genesis modifications
	genesisModify(clip, opts, g)

	if !opts.NoMessage {
		// Modifications for new messages
		g.RunFn(handlerModify(clip, opts))
		g.RunFn(protoTxModify(clip, opts))
		g.RunFn(typesCodecModify(clip, opts))
		g.RunFn(clientCliTxModify(clip, opts))
		g.RunFn(moduleSimulationModify(clip, opts))

		// Messages template
		if err := typed.Box(messagesTemplate, opts, g); err != nil {
			return nil, err
		}
	}

	g.RunFn(frontendSrcStoreAppModify(clip, opts))

	return g, typed.Box(componentTemplate, opts, g)
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
		replacementRPC := fmt.Sprintf(templateRPC, opts.TypeName.UpperCamel)

		if strings.Count(content, typed.PlaceholderProtoTxRPC) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			replacementRPC += typed.PlaceholderProtoTxRPC
			content = clip.Replace(content, typed.PlaceholderProtoTxRPC, replacementRPC)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.ProtoSelectNewServiceMethodPosition,
				clipper.SelectOptions{
					"name": "Msg",
				},
				replacementRPC,
			)
			if err != nil {
				return err
			}
		}

		// Messages
		var createFields string
		for i, field := range opts.Fields {
			createFields += fmt.Sprintf("  %s;\n", field.ProtoType(i+2))
		}
		var updateFields string
		for i, field := range opts.Fields {
			updateFields += fmt.Sprintf("  %s;\n", field.ProtoType(i+3))
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

message MsgCreate%[1]vResponse {
  uint64 id = 1;
}

message MsgUpdate%[1]v {
  string %[2]v = 1;
  uint64 id = 2;
%[4]v}

message MsgUpdate%[1]vResponse {}

message MsgDelete%[1]v {
  string %[2]v = 1;
  uint64 id = 2;
}

message MsgDelete%[1]vResponse {}`
		replacementMessages := fmt.Sprintf(templateMessages,
			opts.TypeName.UpperCamel,
			opts.MsgSigner.LowerCamel,
			createFields,
			updateFields,
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

func protoQueryModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "query.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := strings.ReplaceAll(f.String(), `
import "gogoproto/gogo.proto";`, "")

		// Import
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

		// RPC service
		templateRPC := `
  // Queries a %[2]v by id.
	rpc %[1]v(QueryGet%[1]vRequest) returns (QueryGet%[1]vResponse) {
		option (google.api.http).get = "/%[3]v/%[4]v/%[5]v/%[2]v/{id}";
	}

	// Queries a list of %[2]v items.
	rpc %[1]vAll(QueryAll%[1]vRequest) returns (QueryAll%[1]vResponse) {
		option (google.api.http).get = "/%[3]v/%[4]v/%[5]v/%[2]v";
	}
`
		serviceSnippet := fmt.Sprintf(templateRPC,
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

		// Messages
		templateMessages := `

message QueryGet%[1]vRequest {
	uint64 id = 1;
}

message QueryGet%[1]vResponse {
	%[1]v %[1]v = 1 [(gogoproto.nullable) = false];
}

message QueryAll%[1]vRequest {
	cosmos.base.query.v1beta1.PageRequest pagination = 1;
}

message QueryAll%[1]vResponse {
	repeated %[1]v %[1]v = 1 [(gogoproto.nullable) = false];
	cosmos.base.query.v1beta1.PageResponse pagination = 2;
}`
		replacementMessages := fmt.Sprintf(templateMessages,
			opts.TypeName.UpperCamel,
			opts.TypeName.LowerCamel,
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
	%[1]vCountKey= "%[1]v-count-"
)
`, opts.TypeName.UpperCamel)
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

		// Import
		importSnippet := `sdk "github.com/cosmos/cosmos-sdk/types"`
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		// Concrete
		templateConcrete := `
	cdc.RegisterConcrete(&MsgCreate%[1]v{}, "%[2]v/Create%[1]v", nil)
	cdc.RegisterConcrete(&MsgUpdate%[1]v{}, "%[2]v/Update%[1]v", nil)
	cdc.RegisterConcrete(&MsgDelete%[1]v{}, "%[2]v/Delete%[1]v", nil)`
		startOfFunctionSnippet := fmt.Sprintf(templateConcrete, opts.TypeName.UpperCamel, opts.ModuleName)

		if strings.Count(content, typed.Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + typed.Placeholder2
			content = clip.Replace(content, typed.Placeholder2, startOfFunctionSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
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
		startOfFunctionSnippet = fmt.Sprintf(templateInterface, opts.TypeName.UpperCamel)

		if strings.Count(content, typed.Placeholder3) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + typed.Placeholder3
			content = clip.Replace(content, typed.Placeholder3, startOfFunctionSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
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
			content, err = clip.PasteGoBeforeReturnSnippetAt(
				path,
				content,
				snippet,
				clipper.SelectOptions{
					"functionName": "GetTxCmd",
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

func clientCliQueryModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
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
			content = clip.Replace(f.String(), typed.Placeholder, snippet)
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

func frontendSrcStoreAppModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "vue/src/views/Types.vue")
		f, err := r.Disk.Find(path)
		if os.IsNotExist(err) {
			// Skip modification if the app doesn't contain front-end
			return nil
		}
		if err != nil {
			return err
		}
		replacement := fmt.Sprintf(`%[1]v
		<SpType modulePath="%[2]v.%[3]v.%[4]v" moduleType="%[5]v"  />`,
			typed.Placeholder4,
			opts.OwnerName,
			opts.AppName,
			opts.ModuleName,
			opts.TypeName.UpperCamel,
		)
		content := clip.Replace(f.String(), typed.Placeholder4, replacement)
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
