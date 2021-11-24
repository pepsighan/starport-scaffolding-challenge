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
)

// NewStargate returns the generator to scaffold a module inside a Stargate app
func NewStargate(opts *CreateOptions) (*genny.Generator, error) {
	var (
		g = genny.New()

		msgServerTemplate = xgenny.NewEmbedWalker(
			fsMsgServer,
			"msgserver/",
			opts.AppPath,
		)
		genesisTestTemplate = xgenny.NewEmbedWalker(
			fsGenesisTest,
			"genesistest/",
			opts.AppPath,
		)
		stargateTemplate = xgenny.NewEmbedWalker(
			fsStargate,
			"stargate/",
			opts.AppPath,
		)
	)

	if err := g.Box(msgServerTemplate); err != nil {
		return g, err
	}
	if err := g.Box(genesisTestTemplate); err != nil {
		return g, err
	}
	if err := g.Box(stargateTemplate); err != nil {
		return g, err
	}
	ctx := plush.NewContext()
	ctx.Set("moduleName", opts.ModuleName)
	ctx.Set("modulePath", opts.ModulePath)
	ctx.Set("appName", opts.AppName)
	ctx.Set("ownerName", opts.OwnerName)
	ctx.Set("dependencies", opts.Dependencies)
	ctx.Set("params", opts.Params)
	ctx.Set("isIBC", opts.IsIBC)

	// Used for proto package name
	ctx.Set("formatOwnerName", xstrings.FormatUsername)

	plushhelpers.ExtendPlushContext(ctx)
	g.Transformer(plushgen.Transformer(ctx))
	g.Transformer(genny.Replace("{{moduleName}}", opts.ModuleName))

	gSimapp, err := AddSimulation(opts.AppPath, opts.ModulePath, opts.ModuleName, opts.Params...)
	if err != nil {
		return g, err
	}
	g.Merge(gSimapp)

	return g, nil
}

// NewStargateAppModify returns generator with modifications required to register a module in the app.
func NewStargateAppModify(clip *clipper.Clipper, opts *CreateOptions) *genny.Generator {
	g := genny.New()
	g.RunFn(appModifyStargate(clip, opts))
	if opts.IsIBC {
		g.RunFn(appIBCModify(clip, opts))
	}
	return g
}

// app.go modification on Stargate when creating a module
func appModifyStargate(clip *clipper.Clipper, opts *CreateOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, module.PathAppGo)
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Import
		template := `%[1]vmodule "%[2]v/x/%[1]v"
	%[1]vmodulekeeper "%[2]v/x/%[1]v/keeper"
	%[1]vmoduletypes "%[2]v/x/%[1]v/types"`
		snippet := fmt.Sprintf(template, opts.ModuleName, opts.ModulePath)
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), snippet)
		if err != nil {
			return err
		}

		// ModuleBasic
		template = `%[1]vmodule.AppModuleBasic{}`
		snippet = fmt.Sprintf(template, opts.ModuleName)

		if strings.Count(content, module.PlaceholderSgAppModuleBasic) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += ",\n" + module.PlaceholderSgAppModuleBasic
			content = clip.Replace(content, module.PlaceholderSgAppModuleBasic, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningFunctionNewArgumentSnippetAt(
				path,
				content,
				snippet,
				clipper.SelectOptions{
					"functionName": "newModuleBasics",
				},
			)
			if err != nil {
				return err
			}
		}

		// Keeper declaration
		var scopedKeeperDeclaration string
		if opts.IsIBC {
			// Scoped keeper declaration for IBC module
			// We set this placeholder so it is modified by the IBC module scaffolder
			scopedKeeperDeclaration = module.PlaceholderIBCAppScopedKeeperDeclaration
		}
		template = `
		%[2]v
		%[3]vKeeper %[1]vmodulekeeper.Keeper
`
		snippet = fmt.Sprintf(
			template,
			opts.ModuleName,
			scopedKeeperDeclaration,
			strings.Title(opts.ModuleName),
		)

		if strings.Count(content, module.PlaceholderSgAppKeeperDeclaration) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += module.PlaceholderSgAppKeeperDeclaration
			content = clip.Replace(content, module.PlaceholderSgAppKeeperDeclaration, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.GoSelectStructNewFieldPosition,
				clipper.SelectOptions{
					"structName": "App",
				},
				snippet,
			)
			if err != nil {
				return err
			}
		}

		// Store key
		template = `%[1]vmoduletypes.StoreKey`
		snippet = fmt.Sprintf(template, opts.ModuleName)

		if strings.Count(content, module.PlaceholderSgAppStoreKey) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += ",\n" + module.PlaceholderSgAppStoreKey
			content = clip.Replace(content, module.PlaceholderSgAppStoreKey, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningFunctionNewArgumentSnippetAt(
				path,
				content,
				snippet,
				clipper.SelectOptions{
					"functionName": "newAppKVStoreKeys",
				},
			)
			if err != nil {
				return err
			}
		}

		// Module dependencies
		var depArgs string
		for _, dep := range opts.Dependencies {
			depArgs = fmt.Sprintf("%sapp.%s,\n", depArgs, dep.KeeperName)

			// If bank is a dependency, add account permissions to the module
			if dep.Name == "bank" {
				template = `%[1]vmoduletypes.ModuleName: {authtypes.Minter, authtypes.Burner, authtypes.Staking}`

				snippet = fmt.Sprintf(
					template,
					opts.ModuleName,
				)

				if strings.Count(content, module.PlaceholderSgAppMaccPerms) != 0 {
					// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
					snippet += ",\n" + module.PlaceholderSgAppMaccPerms
					content = clip.Replace(content, module.PlaceholderSgAppMaccPerms, snippet)
				} else {
					// And for newer codebase, we use clipper mechanism.
					content, err = clip.PasteGoReturningCompositeNewArgumentSnippetAt(
						path,
						content,
						snippet,
						clipper.SelectOptions{
							"functionName": "moduleAccountPermissions",
						},
					)
					if err != nil {
						return err
					}
				}
			}
		}

		// Keeper definition
		var scopedKeeperDefinition string
		var ibcKeeperArgument string
		if opts.IsIBC {
			// Scoped keeper definition for IBC module
			// We set this placeholder so it is modified by the IBC module scaffolder
			scopedKeeperDefinition = module.PlaceholderIBCAppScopedKeeperDefinition
			ibcKeeperArgument = module.PlaceholderIBCAppKeeperArgument
		}
		template = `%[3]v
		app.%[5]vKeeper = *%[2]vmodulekeeper.NewKeeper(
			appCodec,
			keys[%[2]vmoduletypes.StoreKey],
			keys[%[2]vmoduletypes.MemStoreKey],
			app.GetSubspace(%[2]vmoduletypes.ModuleName),
			%[4]v
			%[6]v)
		%[2]vModule := %[2]vmodule.NewAppModule(appCodec, app.%[5]vKeeper, app.AccountKeeper, app.BankKeeper)

		%[1]v`
		snippet = fmt.Sprintf(
			template,
			module.PlaceholderSgAppKeeperDefinition,
			opts.ModuleName,
			scopedKeeperDefinition,
			ibcKeeperArgument,
			strings.Title(opts.ModuleName),
			depArgs,
		)
		content = clip.Replace(content, module.PlaceholderSgAppKeeperDefinition, snippet)

		// App Module
		template = `%[2]vModule,
%[1]v`
		snippet = fmt.Sprintf(template, module.PlaceholderSgAppAppModule, opts.ModuleName)
		content = clip.ReplaceAll(content, module.PlaceholderSgAppAppModule, snippet)

		// Init genesis
		template = `%[1]vmoduletypes.ModuleName`
		snippet = fmt.Sprintf(template, opts.ModuleName)
		if strings.Count(content, module.PlaceholderSgAppInitGenesis) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += ",\n" + module.PlaceholderSgAppInitGenesis
			content = clip.Replace(content, module.PlaceholderSgAppInitGenesis, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningCompositeNewArgumentSnippetAt(
				path,
				content,
				snippet,
				clipper.SelectOptions{
					"functionName": "orderedInitGenesisModuleNames",
				},
			)
			if err != nil {
				return err
			}
		}

		// Param subspace
		template = `paramsKeeper.Subspace(%[1]vmoduletypes.ModuleName)`
		snippet = fmt.Sprintf(template, opts.ModuleName)

		if strings.Count(content, module.PlaceholderSgAppParamSubspace) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += "\n" + module.PlaceholderSgAppParamSubspace
			content = clip.Replace(content, module.PlaceholderSgAppParamSubspace, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, snippet, clipper.SelectOptions{
				"functionName": "initParamsKeeper",
			})
			if err != nil {
				return err
			}
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
