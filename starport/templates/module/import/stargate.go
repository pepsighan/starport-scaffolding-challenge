package moduleimport

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/gobuffalo/plush"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/placeholder"
	"github.com/tendermint/starport/starport/templates/field/plushhelpers"
	"github.com/tendermint/starport/starport/templates/module"
)

// NewStargate returns the generator to scaffold code to import wasm module inside a Stargate app
func NewStargate(replacer placeholder.Replacer, opts *ImportOptions) (*genny.Generator, error) {
	g := genny.New()
	g.RunFn(appModifyStargate(replacer, opts))
	g.RunFn(cmdModifyStargate(replacer, opts))

	ctx := plush.NewContext()
	ctx.Set("AppName", opts.AppName)
	plushhelpers.ExtendPlushContext(ctx)

	return g, nil
}

// app.go modification on Stargate when importing wasm
func appModifyStargate(replacer placeholder.Replacer, opts *ImportOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, module.PathAppGo)
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		importSnippet := `"github.com/tendermint/spm-extras/wasmcmd"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmclient "github.com/CosmWasm/wasmd/x/wasm/client"`
		content, err := clipper.PasteGoImportSnippetAt(path, f.String(), importSnippet)
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
		content, err = clipper.PasteCodeSnippetAt(
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
		content = replacer.Replace(content, module.PlaceholderSgAppGovProposalHandlers, replacementProposalHandlers)

		templateModuleBasic := `wasm.AppModuleBasic{}`
		if strings.Count(content, module.PlaceholderSgAppModuleBasic) != 0 {
			// Use the older placeholder mechanism for older codebase.
			templateModuleBasic += ",\n" + module.PlaceholderSgAppModuleBasic
			content = replacer.Replace(content, module.PlaceholderSgAppModuleBasic, templateModuleBasic)
		} else {
			// Use the clipper based code generation for newer codebase.
			content, err = clipper.PasteGoReturningFunctionNewArgumentSnippetAt(
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
		content, err = clipper.PasteCodeSnippetAt(
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

		templateDeclaration := `%[1]v
		scopedWasmKeeper := app.CapabilityKeeper.ScopeToModule(wasm.ModuleName)
		`
		snippet := fmt.Sprintf(templateDeclaration, module.PlaceholderSgAppScopedKeeper)
		content = replacer.Replace(content, module.PlaceholderSgAppScopedKeeper, snippet)

		beforeInitReturnSnippet := `app.scopedWasmKeeper = scopedWasmKeeper`
		content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, beforeInitReturnSnippet, clipper.SelectOptions{
			"functionName": "New",
		})
		if err != nil {
			return err
		}

		snippet = `wasm.StoreKey`
		if strings.Count(content, module.PlaceholderSgAppStoreKey) != 0 {
			// Use the older placeholder mechanism for older codebase.
			snippet += ",\n" + module.PlaceholderSgAppStoreKey
			content = replacer.Replace(content, module.PlaceholderSgAppStoreKey, snippet)
		} else {
			// Use the clipper based code generation for newer codebase.
			content, err = clipper.PasteGoReturningFunctionNewArgumentSnippetAt(
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
		content = replacer.Replace(content, module.PlaceholderSgAppKeeperDefinition, replacementKeeperDefinition)

		templateAppModule := `%[1]v
		wasm.NewAppModule(appCodec, &app.wasmKeeper, app.StakingKeeper),`
		replacementAppModule := fmt.Sprintf(templateAppModule, module.PlaceholderSgAppAppModule)
		content = replacer.Replace(content, module.PlaceholderSgAppAppModule, replacementAppModule)

		snippet = `wasm.ModuleName`
		if strings.Count(content, module.PlaceholderSgAppInitGenesis) != 0 {
			// Use the older placeholder mechanism for older codebase.
			snippet += ",\n" + module.PlaceholderSgAppInitGenesis
			content = replacer.Replace(content, module.PlaceholderSgAppInitGenesis, snippet)
		} else {
			// Use the clipper based code generation for newer codebase.
			content, err = clipper.PasteGoReturningCompositeNewArgumentSnippetAt(
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
		content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
			"functionName": "initParamsKeeper",
		})
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

// main.go modification on Stargate when importing wasm
func cmdModifyStargate(replacer placeholder.Replacer, opts *ImportOptions) genny.RunFn {
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
			// Use the older placeholder mechanism for older codebase.
			templateArgs += ",\n" + module.PlaceholderSgRootArgument
			content = replacer.Replace(content, module.PlaceholderSgRootArgument, templateArgs)
		} else {
			// Use the clipper based code generation for newer codebase.
			content, err = clipper.PasteGoReturningFunctionNewArgumentSnippetAt(
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
		content = replacer.Replace(content, "package main", `package main
import "github.com/tendermint/spm-extras/wasmcmd"`)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
