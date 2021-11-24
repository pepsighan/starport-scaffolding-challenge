package modulecreate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/gobuffalo/plush"
	"github.com/gobuffalo/plushgen"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/xgenny"
	"github.com/tendermint/starport/starport/pkg/xstrings"
	"github.com/tendermint/starport/starport/templates/field/plushhelpers"
	"github.com/tendermint/starport/starport/templates/module"
	"github.com/tendermint/starport/starport/templates/typed"
)

const msgServiceImport = `"github.com/cosmos/cosmos-sdk/types/msgservice"`

// AddMsgServerConventionToLegacyModule add the files and the necessary modifications to an existing module that doesn't support MsgServer convention
// https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-031-msg-service.md
func AddMsgServerConventionToLegacyModule(clip *clipper.Clipper, opts *MsgServerOptions) (*genny.Generator, error) {
	var (
		g        = genny.New()
		template = xgenny.NewEmbedWalker(fsMsgServer, "msgserver/", opts.AppPath)
	)

	g.RunFn(handlerPatch(clip, opts.AppPath, opts.ModuleName))
	g.RunFn(codecPath(clip, opts.AppPath, opts.ModuleName))

	if err := g.Box(template); err != nil {
		return g, err
	}
	ctx := plush.NewContext()
	ctx.Set("moduleName", opts.ModuleName)
	ctx.Set("modulePath", opts.ModulePath)
	ctx.Set("appName", opts.AppName)
	ctx.Set("ownerName", opts.OwnerName)

	// Used for proto package name
	ctx.Set("formatOwnerName", xstrings.FormatUsername)

	plushhelpers.ExtendPlushContext(ctx)
	g.Transformer(plushgen.Transformer(ctx))
	g.Transformer(genny.Replace("{{moduleName}}", opts.ModuleName))
	return g, nil
}

func handlerPatch(clip *clipper.Clipper, appPath, moduleName string) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(appPath, "x", moduleName, "handler.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Add the msg server definition placeholder
		old := "func NewHandler(k keeper.Keeper) sdk.Handler {"
		new := fmt.Sprintf(`%v
%v`, old, typed.PlaceholderHandlerMsgServer)
		content := clip.ReplaceOnce(f.String(), old, new)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func codecPath(clip *clipper.Clipper, appPath, moduleName string) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(appPath, "x", moduleName, "types/codec.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Add msgservice import
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), msgServiceImport)
		if err != nil {
			return err
		}

		// Add RegisterMsgServiceDesc method call
		startOfFunctionSnippet := `
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)`

		if strings.Count(content, module.Placeholder3) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet = module.Placeholder3 + startOfFunctionSnippet
			content = clip.Replace(content, module.Placeholder3, startOfFunctionSnippet)
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
