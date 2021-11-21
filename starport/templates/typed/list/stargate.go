package list

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/placeholder"
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
func NewStargate(replacer placeholder.Replacer, opts *typed.Options) (*genny.Generator, error) {
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

	g.RunFn(protoQueryModify(opts))
	g.RunFn(moduleGRPCGatewayModify(replacer, opts))
	g.RunFn(typesKeyModify(opts))
	g.RunFn(clientCliQueryModify(replacer, opts))

	// Genesis modifications
	genesisModify(replacer, opts, g)

	if !opts.NoMessage {
		// Modifications for new messages
		g.RunFn(handlerModify(replacer, opts))
		g.RunFn(protoTxModify(opts))
		g.RunFn(typesCodecModify(replacer, opts))
		g.RunFn(clientCliTxModify(replacer, opts))
		g.RunFn(moduleSimulationModify(replacer, opts))

		// Messages template
		if err := typed.Box(messagesTemplate, opts, g); err != nil {
			return nil, err
		}
	}

	g.RunFn(frontendSrcStoreAppModify(replacer, opts))

	return g, typed.Box(componentTemplate, opts, g)
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

func protoTxModify(opts *typed.Options) genny.RunFn {
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
		replacementRPC := fmt.Sprintf(templateRPC, opts.TypeName.UpperCamel)
		content, err = clipper.PasteCodeSnippetAt(
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

			content, err = clipper.PasteProtoImportSnippetAt(path, content, importModule)
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

func protoQueryModify(opts *typed.Options) genny.RunFn {
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

		content, err = clipper.PasteProtoImportSnippetAt(path, content, importString)
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
		replacementRPC := fmt.Sprintf(templateRPC,
			opts.TypeName.UpperCamel,
			opts.TypeName.LowerCamel,
			opts.OwnerName,
			opts.AppName,
			opts.ModuleName,
		)
		content, err = clipper.PasteCodeSnippetAt(
			path,
			content,
			clipper.ProtoSelectNewServiceMethodPosition,
			clipper.SelectOptions{
				"name": "Query",
			},
			replacementRPC,
		)
		if err != nil {
			return err
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

func typesCodecModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/codec.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Import
		importSnippet := `sdk "github.com/cosmos/cosmos-sdk/types"`
		content, err := clipper.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		// Concrete
		templateConcrete := `
	cdc.RegisterConcrete(&MsgCreate%[1]v{}, "%[2]v/Create%[1]v", nil)
	cdc.RegisterConcrete(&MsgUpdate%[1]v{}, "%[2]v/Update%[1]v", nil)
	cdc.RegisterConcrete(&MsgDelete%[1]v{}, "%[2]v/Delete%[1]v", nil)`
		startOfFunctionSnippet := fmt.Sprintf(templateConcrete, opts.TypeName.UpperCamel, opts.ModuleName)
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

		// Interface
		templateInterface := `
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreate%[1]v{},
		&MsgUpdate%[1]v{},
		&MsgDelete%[1]v{},
	)`
		startOfFunctionSnippet = fmt.Sprintf(templateInterface, opts.TypeName.UpperCamel)
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
		template := `cmd.AddCommand(CmdCreate%[1]v())
	cmd.AddCommand(CmdUpdate%[1]v())
	cmd.AddCommand(CmdDelete%[1]v())`
		snippet := fmt.Sprintf(template, opts.TypeName.UpperCamel)
		content, err := clipper.PasteGoBeforeReturnSnippetAt(
			path,
			f.String(),
			snippet,
			clipper.SelectOptions{
				"functionName": "GetTxCmd",
			},
		)
		if err != nil {
			return err
		}

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
		template := `cmd.AddCommand(CmdList%[1]v())
	cmd.AddCommand(CmdShow%[1]v())`
		snippet := fmt.Sprintf(template,
			opts.TypeName.UpperCamel,
		)
		content, err := clipper.PasteGoBeforeReturnSnippetAt(path, f.String(), snippet, clipper.SelectOptions{
			"functionName": "GetQueryCmd",
		})
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func frontendSrcStoreAppModify(replacer placeholder.Replacer, opts *typed.Options) genny.RunFn {
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
		content := replacer.Replace(f.String(), typed.Placeholder4, replacement)
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
