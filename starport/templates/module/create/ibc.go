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

// NewIBC returns the generator to scaffold the implementation of the IBCModule interface inside a module
func NewIBC(clip *clipper.Clipper, opts *CreateOptions) (*genny.Generator, error) {
	var (
		g        = genny.New()
		template = xgenny.NewEmbedWalker(fsIBC, "ibc/", opts.AppPath)
	)

	g.RunFn(genesisModify(clip, opts))
	g.RunFn(genesisTypesModify(clip, opts))
	g.RunFn(genesisProtoModify(clip, opts))
	g.RunFn(keysModify(clip, opts))

	if err := g.Box(template); err != nil {
		return g, err
	}
	ctx := plush.NewContext()
	ctx.Set("moduleName", opts.ModuleName)
	ctx.Set("modulePath", opts.ModulePath)
	ctx.Set("appName", opts.AppName)
	ctx.Set("ownerName", opts.OwnerName)
	ctx.Set("ibcOrdering", opts.IBCOrdering)
	ctx.Set("dependencies", opts.Dependencies)

	// Used for proto package name
	ctx.Set("formatOwnerName", xstrings.FormatUsername)

	plushhelpers.ExtendPlushContext(ctx)
	g.Transformer(plushgen.Transformer(ctx))
	g.Transformer(genny.Replace("{{moduleName}}", opts.ModuleName))
	return g, nil
}

func genesisModify(clip *clipper.Clipper, opts *CreateOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "genesis.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		// Genesis init
		initSnippet := `
	k.SetPort(ctx, genState.PortId)
	// Only try to bind to port if it is not already bound, since we may already own
	// port capability from capability InitGenesis
	if !k.IsBound(ctx, genState.PortId) {
		// module binds to the port on InitChain
		// and claims the returned capability
		err := k.BindPort(ctx, genState.PortId)
		if err != nil {
			panic("could not claim port capability: " + err.Error())
		}
	}`

		if strings.Count(content, typed.PlaceholderGenesisModuleInit) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			initSnippet = typed.PlaceholderGenesisModuleInit + initSnippet
			content = clip.Replace(content, typed.PlaceholderGenesisModuleInit, initSnippet)

		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteCodeSnippetAt(
				path,
				content,
				clipper.GoSelectStartOfFunctionPosition,
				clipper.SelectOptions{
					"functionName": "InitGenesis",
				},
				initSnippet,
			)
			if err != nil {
				return err
			}
		}

		// Genesis export
		templateExport := `genesis.PortId = k.GetPort(ctx)`
		if strings.Count(content, typed.PlaceholderGenesisModuleExport) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			templateExport += "\n" + typed.PlaceholderGenesisModuleExport
			content = clip.Replace(content, typed.PlaceholderGenesisModuleExport, templateExport)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, templateExport, clipper.SelectOptions{
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

func genesisTypesModify(clip *clipper.Clipper, opts *CreateOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/genesis.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Import
		importSnippet := `host "github.com/cosmos/ibc-go/modules/core/24-host"`
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		// Default genesis
		templateDefault := `PortId: PortID`

		if strings.Count(content, typed.PlaceholderGenesisTypesDefault) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			templateDefault += "\n" + typed.PlaceholderGenesisTypesDefault
			content = clip.Replace(content, typed.PlaceholderGenesisTypesDefault, templateDefault)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoReturningCompositeNewArgumentSnippetAt(
				path,
				content,
				templateDefault,
				clipper.SelectOptions{
					"functionName": "DefaultGenesis",
				},
			)
			if err != nil {
				return err
			}
		}

		// Validate genesis
		// PlaceholderIBCGenesisTypeValidate
		beforeReturnSnippet := `if err := host.PortIdentifierValidator(gs.PortId); err != nil {
		return err
	}`

		if strings.Count(content, typed.PlaceholderGenesisTypesValidate) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			beforeReturnSnippet += "\n" + typed.PlaceholderGenesisTypesValidate
			content = clip.Replace(content, typed.PlaceholderGenesisTypesValidate, beforeReturnSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
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

func genesisProtoModify(clip *clipper.Clipper, opts *CreateOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "genesis.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		// Determine the new field number
		snippet := `  string port_id = %v;
`

		if strings.Count(content, typed.PlaceholderGenesisProtoState) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += typed.PlaceholderGenesisProtoState
			content = clip.Replace(content, typed.PlaceholderGenesisProtoState, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGeneratedCodeSnippetAt(
				path,
				content,
				clipper.ProtoSelectNewMessageFieldPosition,
				clipper.SelectOptions{
					"name": "GenesisState",
				},
				func(data interface{}) string {
					highestNumber := data.(clipper.ProtoNewMessageFieldPositionData).HighestFieldNumber
					return fmt.Sprintf(snippet, highestNumber+1)
				},
			)
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func keysModify(clip *clipper.Clipper, opts *CreateOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/keys.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Append version and the port id in keys
		templateName := `
const (
	// Version defines the current version the IBC module supports
	Version = "%[1]v-1"
	
	// PortID is the default port id that module binds to
	PortID = "%[1]v"
)`
		constSnippet := fmt.Sprintf(templateName, opts.ModuleName)
		content, err := clip.PasteCodeSnippetAt(
			path,
			f.String(),
			clipper.GoSelectNewGlobalPosition,
			nil,
			constSnippet,
		)

		// PlaceholderIBCKeysPort
		templatePort := `
var (
	// PortKey defines the key to store the port id in store
	PortKey = KeyPrefix("%[1]v-port-")
)`
		varSnippet := fmt.Sprintf(templatePort, opts.ModuleName)
		content, err = clip.PasteCodeSnippetAt(
			path,
			content,
			clipper.GoSelectNewGlobalPosition,
			nil,
			varSnippet,
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func appIBCModify(clip *clipper.Clipper, opts *CreateOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, module.PathAppGo)
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Add route to IBC router
		templateRouter := `ibcRouter.AddRoute(%[2]vmoduletypes.ModuleName, %[2]vModule)
%[1]v`
		replacementRouter := fmt.Sprintf(
			templateRouter,
			module.PlaceholderIBCAppRouter,
			opts.ModuleName,
		)
		content := clip.Replace(f.String(), module.PlaceholderIBCAppRouter, replacementRouter)

		// Scoped keeper declaration for the module
		templateScopedKeeperDeclaration := `Scoped%[1]vKeeper capabilitykeeper.ScopedKeeper`
		replacementScopedKeeperDeclaration := fmt.Sprintf(templateScopedKeeperDeclaration, strings.Title(opts.ModuleName))
		content = clip.Replace(content, module.PlaceholderIBCAppScopedKeeperDeclaration, replacementScopedKeeperDeclaration)

		// Scoped keeper definition
		templateScopedKeeperDefinition := `scoped%[1]vKeeper := app.CapabilityKeeper.ScopeToModule(%[2]vmoduletypes.ModuleName)
app.Scoped%[1]vKeeper = scoped%[1]vKeeper`
		replacementScopedKeeperDefinition := fmt.Sprintf(
			templateScopedKeeperDefinition,
			strings.Title(opts.ModuleName),
			opts.ModuleName,
		)
		content = clip.Replace(content, module.PlaceholderIBCAppScopedKeeperDefinition, replacementScopedKeeperDefinition)

		// New argument passed to the module keeper
		templateKeeperArgument := `app.IBCKeeper.ChannelKeeper,
&app.IBCKeeper.PortKeeper,
scoped%[1]vKeeper,`
		replacementKeeperArgument := fmt.Sprintf(
			templateKeeperArgument,
			strings.Title(opts.ModuleName),
		)
		content = clip.Replace(content, module.PlaceholderIBCAppKeeperArgument, replacementKeeperArgument)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
