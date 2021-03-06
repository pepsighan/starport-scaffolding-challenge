package message

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/xgenny"
	"github.com/tendermint/starport/starport/templates/typed"
)

// NewStargate returns the generator to scaffold a empty message in a Stargate module
func NewStargate(clip *clipper.Clipper, opts *Options) (*genny.Generator, error) {
	g := genny.New()

	g.RunFn(handlerModify(clip, opts))
	g.RunFn(protoTxRPCModify(clip, opts))
	g.RunFn(protoTxMessageModify(clip, opts))
	g.RunFn(typesCodecModify(clip, opts))
	g.RunFn(clientCliTxModify(clip, opts))
	g.RunFn(moduleSimulationModify(clip, opts))

	template := xgenny.NewEmbedWalker(
		fsStargate,
		"stargate/",
		opts.AppPath,
	)
	return g, Box(template, opts, g)
}

func handlerModify(clip *clipper.Clipper, opts *Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "handler.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Set once the MsgServer definition if it is not defined yet
		replacementMsgServer := `msgServer := keeper.NewMsgServerImpl(k)`
		content := clip.ReplaceOnce(f.String(), PlaceholderHandlerMsgServer, replacementMsgServer)

		templateHandlers := `case *types.Msg%[2]v:
					res, err := msgServer.%[2]v(sdk.WrapSDKContext(ctx), msg)
					return sdk.WrapServiceResult(ctx, res, err)
%[1]v`
		replacementHandlers := fmt.Sprintf(templateHandlers,
			Placeholder,
			opts.MsgName.UpperCamel,
		)
		content = clip.Replace(content, Placeholder, replacementHandlers)
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func protoTxRPCModify(clip *clipper.Clipper, opts *Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "tx.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()
		template := `  rpc %[1]v(Msg%[1]v) returns (Msg%[1]vResponse);
`
		snippet := fmt.Sprintf(template, opts.MsgName.UpperCamel)

		if strings.Count(content, PlaceholderProtoTxRPC) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += PlaceholderProtoTxRPC
			content = clip.Replace(content, PlaceholderProtoTxRPC, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.ProtoSelectNewServiceMethodPosition,
				clipper.SelectOptions{
					"name": "Msg",
				},
				snippet,
			)
			if err != nil {
				return err
			}
		}
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func protoTxMessageModify(clip *clipper.Clipper, opts *Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "tx.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		var msgFields string
		for i, field := range opts.Fields {
			msgFields += fmt.Sprintf("  %s;\n", field.ProtoType(i+2))
		}
		var resFields string
		for i, field := range opts.ResFields {
			resFields += fmt.Sprintf("  %s;\n", field.ProtoType(i+1))
		}

		template := `

message Msg%[1]v {
  string %[4]v = 1;
%[2]v}

message Msg%[1]vResponse {
%[3]v}`
		replacement := fmt.Sprintf(template,
			opts.MsgName.UpperCamel,
			msgFields,
			resFields,
			opts.MsgSigner.LowerCamel,
		)
		content, err := clip.PasteCodeSnippetAt(
			path,
			f.String(),
			clipper.ProtoSelectLastPosition,
			nil,
			replacement,
		)
		if err != nil {
			return err
		}

		// Ensure custom types are imported
		protoImports := append(opts.ResFields.ProtoImports(), opts.Fields.ProtoImports()...)
		customFields := append(opts.ResFields.Custom(), opts.Fields.Custom()...)
		for _, f := range customFields {
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

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func typesCodecModify(clip *clipper.Clipper, opts *Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/codec.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}
		importSnippet := `sdk "github.com/cosmos/cosmos-sdk/types"`
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		templateRegisterConcrete := `
	cdc.RegisterConcrete(&Msg%[1]v{}, "%[2]v/%[1]v", nil)`
		startOfFunctionSnippet := fmt.Sprintf(
			templateRegisterConcrete,
			opts.MsgName.UpperCamel,
			opts.ModuleName,
		)

		if strings.Count(content, Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + Placeholder2
			content = clip.Replace(content, Placeholder2, startOfFunctionSnippet)
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

		templateRegisterImplementations := `
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&Msg%[1]v{},
	)`
		startOfFunctionSnippet = fmt.Sprintf(
			templateRegisterImplementations,
			opts.MsgName.UpperCamel,
		)

		if strings.Count(content, Placeholder3) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + Placeholder3
			content = clip.Replace(content, Placeholder3, startOfFunctionSnippet)
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

func clientCliTxModify(clip *clipper.Clipper, opts *Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "client/cli/tx.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()
		template := `cmd.AddCommand(Cmd%[1]v())`
		snippet := fmt.Sprintf(template, opts.MsgName.UpperCamel)

		if strings.Count(content, Placeholder) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += "\n" + Placeholder
			content = clip.Replace(content, Placeholder, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, snippet, clipper.SelectOptions{
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

func moduleSimulationModify(clip *clipper.Clipper, opts *Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "module_simulation.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content, err := typed.ModuleSimulationMsgModify(
			clip,
			path,
			f.String(),
			opts.ModuleName,
			opts.MsgName,
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
