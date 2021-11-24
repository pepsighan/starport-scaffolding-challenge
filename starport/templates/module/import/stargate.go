package moduleimport

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/gobuffalo/plush"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/templates/field/plushhelpers"
	"github.com/tendermint/starport/starport/templates/module"
)

// NewStargate returns the generator to scaffold code to import wasm module inside a Stargate app
func NewStargate(clip *clipper.Clipper, opts *ImportOptions) (*genny.Generator, error) {
	g := genny.New()
	g.RunFn(appModifyStargate(clip, opts))
	g.RunFn(cmdModifyStargate(clip, opts))

	ctx := plush.NewContext()
	ctx.Set("AppName", opts.AppName)
	plushhelpers.ExtendPlushContext(ctx)

	return g, nil
}

// app.go modification on Stargate when importing wasm
func appModifyStargate(clip *clipper.Clipper, opts *ImportOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, module.PathAppGo)
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		importSnippet := `"github.com/tendermint/spm-extras/wasmcmd"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmclient "github.com/CosmWasm/wasmd/x/wasm/client"`
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		enabledProposalsSnippet := `
var (
	// If EnabledSpecificProposals is "", and this is "true", then enable all x/wasm proposals.
	// If EnabledSpecificProposals is "", and this is not "true", then disable all x/wasm proposals.
	ProposalsEnabled = "false"
	// If set to non-empty string it must be comma-separated list of values that are all a subset
	// of "EnableAllProposals" (takes precedence over ProposalsEnabled)
	// https://github.com/CosmWasm/wasmd/blob/02a54d33ff2c064f3539ae12d75d027d9c665f05/x/wasm/internal/types/proposal.go#L28-L34
	EnableSpecificProposals = ""
)`
		content, err = clip.PasteCodeSnippetAt(
			path,
			content,
			clipper.GoSelectNewGlobalPosition,
			nil,
			enabledProposalsSnippet,
		)
		if err != nil {
			return err
		}

		templateGovProposalHandlers := `%[1]v
		govProposalHandlers = wasmclient.ProposalHandlers`
		replacementProposalHandlers := fmt.Sprintf(templateGovProposalHandlers, module.PlaceholderSgAppGovProposalHandlers)
		content = clip.Replace(content, module.PlaceholderSgAppGovProposalHandlers, replacementProposalHandlers)

		templateModuleBasic := `wasm.AppModuleBasic{}`
		if strings.Count(content, module.PlaceholderSgAppModuleBasic) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			templateModuleBasic += ",\n" + module.PlaceholderSgAppModuleBasic
			content = clip.Replace(content, module.PlaceholderSgAppModuleBasic, templateModuleBasic)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningFunctionNewArgumentSnippetAt(
				path,
				content,
				templateModuleBasic,
				clipper.SelectOptions{
					"functionName": "newModuleBasics",
				},
			)
			if err != nil {
				return err
			}
		}

		structFieldSnippet := `
	wasmKeeper       wasm.Keeper
	scopedWasmKeeper capabilitykeeper.ScopedKeeper
`

		if strings.Count(content, module.PlaceholderSgAppKeeperDeclaration) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			structFieldSnippet = module.PlaceholderSgAppKeeperDeclaration + structFieldSnippet
			content = clip.Replace(content, module.PlaceholderSgAppKeeperDeclaration, structFieldSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.GoSelectStructNewFieldPosition,
				clipper.SelectOptions{
					"structName": "App",
				},
				structFieldSnippet,
			)
			if err != nil {
				return err
			}
		}

		templateDeclaration := `%[1]v
		scopedWasmKeeper := app.CapabilityKeeper.ScopeToModule(wasm.ModuleName)
		`
		snippet := fmt.Sprintf(templateDeclaration, module.PlaceholderSgAppScopedKeeper)
		content = clip.Replace(content, module.PlaceholderSgAppScopedKeeper, snippet)

		beforeInitReturnSnippet := `app.scopedWasmKeeper = scopedWasmKeeper`
		if strings.Count(content, module.PlaceholderSgAppBeforeInitReturn) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			beforeInitReturnSnippet = module.PlaceholderSgAppBeforeInitReturn + "\n" + beforeInitReturnSnippet
			content = clip.Replace(content, module.PlaceholderSgAppBeforeInitReturn, beforeInitReturnSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, beforeInitReturnSnippet, clipper.SelectOptions{
				"functionName": "New",
			})
			if err != nil {
				return err
			}
		}

		snippet = `wasm.StoreKey`
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

		templateKeeperDefinition := `%[1]v
		wasmDir := filepath.Join(homePath, "wasm")
	
		wasmConfig, err := wasm.ReadWasmConfig(appOpts)
		if err != nil {
			panic("error while reading wasm config: " + err.Error())
		}

		// The last arguments can contain custom message handlers, and custom query handlers,
		// if we want to allow any custom callbacks
		supportedFeatures := "staking"
		app.wasmKeeper = wasm.NewKeeper(
				appCodec,
				keys[wasm.StoreKey],
				app.GetSubspace(wasm.ModuleName),
				app.AccountKeeper,
				app.BankKeeper,
				app.StakingKeeper,
				app.DistrKeeper,
				app.IBCKeeper.ChannelKeeper,
				&app.IBCKeeper.PortKeeper,
				scopedWasmKeeper,
				app.TransferKeeper,
				app.Router(),
				app.GRPCQueryRouter(),
				wasmDir,
				wasmConfig,
				supportedFeatures,
		)
	
		// The gov proposal types can be individually enabled
		enabledProposals := wasmcmd.GetEnabledProposals(ProposalsEnabled, EnableSpecificProposals)
		if len(enabledProposals) != 0 {
			govRouter.AddRoute(wasm.RouterKey, wasm.NewWasmProposalHandler(app.wasmKeeper, enabledProposals))
		}`
		replacementKeeperDefinition := fmt.Sprintf(templateKeeperDefinition, module.PlaceholderSgAppKeeperDefinition)
		content = clip.Replace(content, module.PlaceholderSgAppKeeperDefinition, replacementKeeperDefinition)

		templateAppModule := `%[1]v
		wasm.NewAppModule(appCodec, &app.wasmKeeper, app.StakingKeeper),`
		replacementAppModule := fmt.Sprintf(templateAppModule, module.PlaceholderSgAppAppModule)
		content = clip.Replace(content, module.PlaceholderSgAppAppModule, replacementAppModule)

		snippet = `wasm.ModuleName`
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

		beforeReturnSnippet := `paramsKeeper.Subspace(wasm.ModuleName)`
		if strings.Count(content, module.PlaceholderSgAppParamSubspace) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			beforeReturnSnippet = module.PlaceholderSgAppParamSubspace + "\n" + beforeReturnSnippet
			content = clip.Replace(content, module.PlaceholderSgAppParamSubspace, beforeReturnSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
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

// main.go modification on Stargate when importing wasm
func cmdModifyStargate(clip *clipper.Clipper, opts *ImportOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "cmd", opts.BinaryNamePrefix+"d/main.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		templateArgs := `cosmoscmd.AddSubCmd(wasmcmd.GenesisWasmMsgCmd(app.DefaultNodeHome)),
cosmoscmd.CustomizeStartCmd(wasmcmd.AddModuleInitFlags)`

		if strings.Count(content, module.PlaceholderSgRootArgument) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			templateArgs += ",\n" + module.PlaceholderSgRootArgument
			content = clip.Replace(content, module.PlaceholderSgRootArgument, templateArgs)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningFunctionNewArgumentSnippetAt(
				path,
				content,
				templateArgs,
				clipper.SelectOptions{
					"functionName": "newRootCmd",
				},
			)
			if err != nil {
				return err
			}
		}

		// import spm-extras.
		content = clip.Replace(content, "package main", `package main
import "github.com/tendermint/spm-extras/wasmcmd"`)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
