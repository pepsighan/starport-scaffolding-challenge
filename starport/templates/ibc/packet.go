package ibc

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/gobuffalo/plush"
	"github.com/gobuffalo/plushgen"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/multiformatname"
	"github.com/tendermint/starport/starport/pkg/xgenny"
	"github.com/tendermint/starport/starport/templates/field"
	"github.com/tendermint/starport/starport/templates/field/plushhelpers"
	"github.com/tendermint/starport/starport/templates/module"
	"github.com/tendermint/starport/starport/templates/testutil"
)

var (
	//go:embed packet/component/* packet/component/**/*
	fsPacketComponent embed.FS

	//go:embed packet/messages/* packet/messages/**/*
	fsPacketMessages embed.FS
)

// PacketOptions are options to scaffold a packet in a IBC module
type PacketOptions struct {
	AppName    string
	AppPath    string
	ModuleName string
	ModulePath string
	OwnerName  string
	PacketName multiformatname.Name
	MsgSigner  multiformatname.Name
	Fields     field.Fields
	AckFields  field.Fields
	NoMessage  bool
}

// NewPacket returns the generator to scaffold a packet in an IBC module
func NewPacket(clip *clipper.Clipper, opts *PacketOptions) (*genny.Generator, error) {
	var (
		g = genny.New()

		messagesTemplate = xgenny.NewEmbedWalker(
			fsPacketMessages,
			"packet/messages/",
			opts.AppPath,
		)
		componentTemplate = xgenny.NewEmbedWalker(
			fsPacketComponent,
			"packet/component/",
			opts.AppPath,
		)
	)

	// Add the component
	g.RunFn(moduleModify(clip, opts))
	g.RunFn(protoModify(clip, opts))
	g.RunFn(eventModify(clip, opts))
	if err := g.Box(componentTemplate); err != nil {
		return g, err
	}

	// Add the send message
	if !opts.NoMessage {
		g.RunFn(protoTxModify(clip, opts))
		g.RunFn(handlerTxModify(clip, opts))
		g.RunFn(clientCliTxModify(clip, opts))
		g.RunFn(codecModify(clip, opts))
		if err := g.Box(messagesTemplate); err != nil {
			return g, err
		}
	}

	ctx := plush.NewContext()
	ctx.Set("moduleName", opts.ModuleName)
	ctx.Set("ModulePath", opts.ModulePath)
	ctx.Set("appName", opts.AppName)
	ctx.Set("packetName", opts.PacketName)
	ctx.Set("MsgSigner", opts.MsgSigner)
	ctx.Set("ownerName", opts.OwnerName)
	ctx.Set("fields", opts.Fields)
	ctx.Set("ackFields", opts.AckFields)

	plushhelpers.ExtendPlushContext(ctx)
	g.Transformer(plushgen.Transformer(ctx))
	g.Transformer(genny.Replace("{{moduleName}}", opts.ModuleName))
	g.Transformer(genny.Replace("{{packetName}}", opts.PacketName.Snake))

	// Create the 'testutil' package with the test helpers
	if err := testutil.Register(g, opts.AppPath); err != nil {
		return g, err
	}

	return g, nil
}

func moduleModify(clip *clipper.Clipper, opts *PacketOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "module_ibc.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Recv packet dispatch
		templateRecv := `case *types.%[2]vPacketData_%[3]vPacket:
	packetAck, err := am.keeper.OnRecv%[3]vPacket(ctx, modulePacket, *packet.%[3]vPacket)
	if err != nil {
		ack = channeltypes.NewErrorAcknowledgement(err.Error())
	} else {
		// Encode packet acknowledgment
		packetAckBytes, err := types.ModuleCdc.MarshalJSON(&packetAck)
		if err != nil {
			return channeltypes.NewErrorAcknowledgement(sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error()).Error())
		}
		ack = channeltypes.NewResultAcknowledgement(sdk.MustSortJSON(packetAckBytes))
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventType%[3]vPacket,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyAckSuccess, fmt.Sprintf("%%t", err != nil)),
		),
	)
%[1]v`
		replacementRecv := fmt.Sprintf(
			templateRecv,
			PlaceholderIBCPacketModuleRecv,
			strings.Title(opts.ModuleName),
			opts.PacketName.UpperCamel,
		)
		content := clip.Replace(f.String(), PlaceholderIBCPacketModuleRecv, replacementRecv)

		// Ack packet dispatch
		templateAck := `case *types.%[2]vPacketData_%[3]vPacket:
	err := am.keeper.OnAcknowledgement%[3]vPacket(ctx, modulePacket, *packet.%[3]vPacket, ack)
	if err != nil {
		return nil, err
	}
	eventType = types.EventType%[3]vPacket
%[1]v`
		replacementAck := fmt.Sprintf(
			templateAck,
			PlaceholderIBCPacketModuleAck,
			strings.Title(opts.ModuleName),
			opts.PacketName.UpperCamel,
		)
		content = clip.Replace(content, PlaceholderIBCPacketModuleAck, replacementAck)

		// Timeout packet dispatch
		templateTimeout := `case *types.%[2]vPacketData_%[3]vPacket:
	err := am.keeper.OnTimeout%[3]vPacket(ctx, modulePacket, *packet.%[3]vPacket)
	if err != nil {
		return nil, err
	}
%[1]v`
		replacementTimeout := fmt.Sprintf(
			templateTimeout,
			PlaceholderIBCPacketModuleTimeout,
			strings.Title(opts.ModuleName),
			opts.PacketName.UpperCamel,
		)
		content = clip.Replace(content, PlaceholderIBCPacketModuleTimeout, replacementTimeout)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func protoModify(clip *clipper.Clipper, opts *PacketOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "packet.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		// Add the field in the module packet
		templateField := `  %[1]vPacketData %[2]vPacket = %[3]v;
  `
		if strings.Count(content, PlaceholderIBCPacketProtoField) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			fieldCount := strings.Count(content, PlaceholderIBCPacketProtoFieldNumber)
			replacementField := fmt.Sprintf(
				templateField,
				opts.PacketName.UpperCamel,
				opts.PacketName.LowerCamel,
				fieldCount+2,
			)
			replacementField = PlaceholderIBCPacketProtoField + "\n\t" +
				strings.TrimSpace(replacementField) + " " + PlaceholderIBCPacketProtoFieldNumber
			content = clip.Replace(content, PlaceholderIBCPacketProtoField, replacementField)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clip.PasteGeneratedCodeSnippetAt(
				path,
				content,
				clipper.ProtoSelectNewOneOfFieldPosition,
				clipper.SelectOptions{
					"messageName": fmt.Sprintf("%vPacketData", strings.Title(opts.ModuleName)),
					"oneOfName":   "packet",
				},
				func(data interface{}) string {
					fieldNumber := data.(clipper.ProtoNewOneOfFieldPositionData).HighestFieldNumber
					return fmt.Sprintf(
						templateField,
						opts.PacketName.UpperCamel,
						opts.PacketName.LowerCamel,
						fieldNumber+1,
					)
				},
			)
			if err != nil {
				return err
			}
		}

		// Add the message definition for packet and acknowledgment
		var packetFields string
		for i, fld := range opts.Fields {
			packetFields += fmt.Sprintf("  %s;\n", fld.ProtoType(i+1))
		}

		var ackFields string
		for i, fld := range opts.AckFields {
			ackFields += fmt.Sprintf("  %s;\n", fld.ProtoType(i+1))
		}

		// Ensure custom types are imported
		protoImports := append(opts.Fields.ProtoImports(), opts.AckFields.ProtoImports()...)
		customFields := append(opts.Fields.Custom(), opts.AckFields.Custom()...)
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

		templateMessage := `

// %[1]vPacketData defines a struct for the packet payload
message %[1]vPacketData {
%[2]v}

// %[1]vPacketAck defines a struct for the packet acknowledgment
message %[1]vPacketAck {
%[3]v}`
		replacementMessage := fmt.Sprintf(
			templateMessage,
			opts.PacketName.UpperCamel,
			packetFields,
			ackFields,
		)
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

func eventModify(clip *clipper.Clipper, opts *PacketOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/events_ibc.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		template := `
const EventType%[1]vPacket = "%[2]v_packet"`
		snippet := fmt.Sprintf(
			template,
			opts.PacketName.UpperCamel,
			opts.PacketName.LowerCamel,
		)
		content, err := clip.PasteCodeSnippetAt(path, f.String(), clipper.GoSelectNewGlobalPosition, nil, snippet)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func protoTxModify(clip *clipper.Clipper, opts *PacketOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "tx.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		// RPC
		templateRPC := `  rpc Send%[1]v(MsgSend%[1]v) returns (MsgSend%[1]vResponse);
`
		serviceSnippet := fmt.Sprintf(
			templateRPC,
			opts.PacketName.UpperCamel,
		)

		if strings.Count(content, PlaceholderProtoTxRPC) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			serviceSnippet += PlaceholderProtoTxRPC
			content = clip.Replace(content, PlaceholderProtoTxRPC, serviceSnippet)
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
		var sendFields string
		for i, fld := range opts.Fields {
			sendFields += fmt.Sprintf("  %s;\n", fld.ProtoType(i+5))
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

		// Message
		// TODO: Include timestamp height
		// This addition would include using the type ibc.core.client.v1.Height
		// Ex: https://github.com/cosmos/cosmos-sdk/blob/816306b85addae6350bd380997f2f4bf9dce9471/proto/ibc/applications/transfer/v1/tx.proto
		templateMessage := `

message MsgSend%[1]v {
  string %[2]v = 1;
  string port = 2;
  string channelID = 3;
  uint64 timeoutTimestamp = 4;
%[3]v}

message MsgSend%[1]vResponse {
}
`
		replacementMessage := fmt.Sprintf(
			templateMessage,
			opts.PacketName.UpperCamel,
			opts.MsgSigner.LowerCamel,
			sendFields,
		)
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

func handlerTxModify(clip *clipper.Clipper, opts *PacketOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "handler.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Set once the MsgServer definition if it is not defined yet
		replacementMsgServer := `msgServer := keeper.NewMsgServerImpl(k)`
		content := clip.ReplaceOnce(f.String(), PlaceholderHandlerMsgServer, replacementMsgServer)

		templateHandlers := `case *types.MsgSend%[2]v:
					res, err := msgServer.Send%[2]v(sdk.WrapSDKContext(ctx), msg)
					return sdk.WrapServiceResult(ctx, res, err)
%[1]v`
		replacementHandlers := fmt.Sprintf(templateHandlers,
			Placeholder,
			opts.PacketName.UpperCamel,
		)
		content = clip.Replace(content, Placeholder, replacementHandlers)
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func clientCliTxModify(clip *clipper.Clipper, opts *PacketOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "client/cli/tx.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := f.String()

		template := `cmd.AddCommand(CmdSend%[1]v())`
		snippet := fmt.Sprintf(template, opts.PacketName.UpperCamel)

		if strings.Count(content, Placeholder) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += "\n" + Placeholder
			content = clip.Replace(f.String(), Placeholder, snippet)
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

func codecModify(clip *clipper.Clipper, opts *PacketOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/codec.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Set import if not set yet
		importSnippet := `sdk "github.com/cosmos/cosmos-sdk/types"`
		content, err := clip.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		// Register the module packet
		templateRegistry := `
	cdc.RegisterConcrete(&MsgSend%[1]v{}, "%[2]v/Send%[1]v", nil)`
		startOfFunctionSnippet := fmt.Sprintf(
			templateRegistry,
			opts.PacketName.UpperCamel,
			opts.ModuleName,
		)

		if strings.Count(content, module.Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + module.Placeholder2
			content = clip.Replace(content, module.Placeholder2, startOfFunctionSnippet)
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

		// Register the module packet interface
		templateInterface := `
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSend%[1]v{},
	)`
		startOfFunctionSnippet = fmt.Sprintf(templateInterface, opts.PacketName.UpperCamel)

		if strings.Count(content, module.Placeholder3) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + module.Placeholder3
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
