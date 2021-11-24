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
	"github.com/tendermint/starport/starport/pkg/placeholder"
	"github.com/tendermint/starport/starport/pkg/xgenny"
	"github.com/tendermint/starport/starport/pkg/xstrings"
	"github.com/tendermint/starport/starport/templates/field/plushhelpers"
	"github.com/tendermint/starport/starport/templates/testutil"
)

var (
	//go:embed oracle/* oracle/**/*
	fsOracle embed.FS
)

// OracleOptions are options to scaffold an oracle query in a IBC module
type OracleOptions struct {
	AppName    string
	AppPath    string
	ModuleName string
	ModulePath string
	OwnerName  string
	QueryName  multiformatname.Name
	MsgSigner  multiformatname.Name
}

// NewOracle returns the generator to scaffold the implementation of the Oracle interface inside a module
func NewOracle(replacer placeholder.Replacer, opts *OracleOptions) (*genny.Generator, error) {
	g := genny.New()

	template := xgenny.NewEmbedWalker(fsOracle, "oracle/", opts.AppPath)

	g.RunFn(moduleOracleModify(replacer, opts))
	g.RunFn(protoQueryOracleModify(replacer, opts))
	g.RunFn(protoTxOracleModify(replacer, opts))
	g.RunFn(handlerTxOracleModify(replacer, opts))
	g.RunFn(clientCliQueryOracleModify(replacer, opts))
	g.RunFn(clientCliTxOracleModify(replacer, opts))
	g.RunFn(codecOracleModify(replacer, opts))

	ctx := plush.NewContext()
	ctx.Set("moduleName", opts.ModuleName)
	ctx.Set("ModulePath", opts.ModulePath)
	ctx.Set("appName", opts.AppName)
	ctx.Set("ownerName", opts.OwnerName)
	ctx.Set("queryName", opts.QueryName)
	ctx.Set("MsgSigner", opts.MsgSigner)

	// Used for proto package name
	ctx.Set("formatOwnerName", xstrings.FormatUsername)

	plushhelpers.ExtendPlushContext(ctx)
	g.Transformer(plushgen.Transformer(ctx))
	g.Transformer(genny.Replace("{{moduleName}}", opts.ModuleName))
	g.Transformer(genny.Replace("{{queryName}}", opts.QueryName.Snake))

	// Create the 'testutil' package with the test helpers
	if err := testutil.Register(g, opts.AppPath); err != nil {
		return g, err
	}

	if err := xgenny.Box(g, template); err != nil {
		return g, err
	}

	g.RunFn(packetHandlerOracleModify(replacer, opts))

	return g, nil
}

func moduleOracleModify(replacer placeholder.Replacer, opts *OracleOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "module_ibc.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Recv packet dispatch
		templateRecv := `oracleAck, err := am.handleOraclePacket(ctx, modulePacket)
	if err != nil {
		return channeltypes.NewErrorAcknowledgement(sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, "cannot unmarshal packet data: "+err.Error()).Error())
	} else if ack != oracleAck {
		return oracleAck
	}
	%[1]v`
		replacementRecv := fmt.Sprintf(templateRecv, PlaceholderOraclePacketModuleRecv)
		content := replacer.ReplaceOnce(f.String(), PlaceholderOraclePacketModuleRecv, replacementRecv)

		// Ack packet dispatch
		templateAck := `sdkResult, err := am.handleOracleAcknowledgment(ctx, ack, modulePacket)
	if err != nil {
		return nil, err
	}
	if sdkResult != nil {
		sdkResult.Events = ctx.EventManager().Events().ToABCIEvents()
		return sdkResult, nil
	}
	%[1]v`
		replacementAck := fmt.Sprintf(templateAck, PlaceholderOraclePacketModuleAck)
		content = replacer.ReplaceOnce(content, PlaceholderOraclePacketModuleAck, replacementAck)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func protoQueryOracleModify(replacer placeholder.Replacer, opts *OracleOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "query.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Import the type
		templateImport := `
import "%[1]v/%[2]v.proto";`
		importString := fmt.Sprintf(templateImport, opts.ModuleName, opts.QueryName.Snake)
		content, err := clipper.PasteProtoImportSnippetAt(path, f.String(), importString)
		if err != nil {
			return err
		}

		// Add the service
		templateService := `
  // %[1]vResult defines a rpc handler method for Msg%[1]vData.
  rpc %[1]vResult(Query%[1]vRequest) returns (Query%[1]vResponse) {
    option (google.api.http).get = "/%[3]v/%[4]v/%[2]v_result/{request_id}";
  }

  // Last%[1]vId query the last %[1]v result id
  rpc Last%[1]vId(QueryLast%[1]vIdRequest) returns (QueryLast%[1]vIdResponse) {
    option (google.api.http).get = "/%[3]v/%[4]v/last_%[2]v_id";
  }
`
		serviceSnippet := fmt.Sprintf(templateService,
			opts.QueryName.UpperCamel,
			opts.QueryName.Snake,
			opts.AppName,
			opts.ModuleName,
		)

		if strings.Count(content, Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			serviceSnippet += Placeholder2
			content = replacer.Replace(content, Placeholder2, serviceSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteCodeSnippetAt(
				path,
				content,
				clipper.ProtoSelectNewServiceMethodPosition,
				clipper.SelectOptions{
					"name": "Query",
				},
				serviceSnippet,
			)
		}

		// Add the service messages
		templateMessage := `

message Query%[1]vRequest {int64 request_id = 1;}

message Query%[1]vResponse {
  %[1]vResult result = 1;
}

message QueryLast%[1]vIdRequest {}

message QueryLast%[1]vIdResponse {int64 request_id = 1;}`
		messagesSnippet := fmt.Sprintf(templateMessage, opts.QueryName.UpperCamel)
		content, err = clipper.PasteCodeSnippetAt(
			path,
			content,
			clipper.ProtoSelectLastPosition,
			nil,
			messagesSnippet,
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func protoTxOracleModify(replacer placeholder.Replacer, opts *OracleOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "tx.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content := strings.ReplaceAll(f.String(), `
import "gogoproto/gogo.proto";`, "")
		content = strings.ReplaceAll(content, `
import "cosmos/base/v1beta1/coin.proto";`, "")

		// Import
		templateImport := `
import "gogoproto/gogo.proto";
import "cosmos/base/v1beta1/coin.proto";
import "%[1]v/%[2]v.proto";`
		importString := fmt.Sprintf(templateImport, opts.ModuleName, opts.QueryName.Snake)
		content, err = clipper.PasteProtoImportSnippetAt(path, content, importString)
		if err != nil {
			return err
		}

		// RPC
		templateRPC := `  rpc %[1]vData(Msg%[1]vData) returns (Msg%[1]vDataResponse);
`
		serviceSnippet := fmt.Sprintf(templateRPC, opts.QueryName.UpperCamel)

		if strings.Count(content, PlaceholderProtoTxRPC) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			serviceSnippet += PlaceholderProtoTxRPC
			content = replacer.Replace(content, PlaceholderProtoTxRPC, serviceSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			content, err = clipper.PasteCodeSnippetAt(
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

		templateMessage := `

message Msg%[1]vData {
  string %[2]v = 1;
  uint64 oracle_script_id = 2 [
    (gogoproto.customname) = "OracleScriptID",
    (gogoproto.moretags) = "yaml:\"oracle_script_id\""
  ];
  string source_channel = 3;
  %[1]vCallData calldata = 4;
  uint64 ask_count = 5;
  uint64 min_count = 6;
  repeated cosmos.base.v1beta1.Coin fee_limit = 7 [
    (gogoproto.nullable) = false,
    (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
  ];
  uint64 prepare_gas = 8;
  uint64 execute_gas = 9;
  string client_id = 10 [(gogoproto.customname) = "ClientID"];
}

message Msg%[1]vDataResponse {
}`
		messagesSnippet := fmt.Sprintf(templateMessage,
			opts.QueryName.UpperCamel,
			opts.MsgSigner.LowerCamel,
		)
		content, err = clipper.PasteCodeSnippetAt(
			path,
			content,
			clipper.ProtoSelectLastPosition,
			nil,
			messagesSnippet,
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func handlerTxOracleModify(replacer placeholder.Replacer, opts *OracleOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "handler.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Set once the MsgServer definition if it is not defined yet
		replacementMsgServer := `msgServer := keeper.NewMsgServerImpl(k)`
		content := replacer.ReplaceOnce(f.String(), PlaceholderHandlerMsgServer, replacementMsgServer)

		templateHandlers := `case *types.Msg%[2]vData:
					res, err := msgServer.%[2]vData(sdk.WrapSDKContext(ctx), msg)
					return sdk.WrapServiceResult(ctx, res, err)
%[1]v`
		replacementHandlers := fmt.Sprintf(templateHandlers, Placeholder, opts.QueryName.UpperCamel)
		content = replacer.Replace(content, Placeholder, replacementHandlers)
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func clientCliQueryOracleModify(replacer placeholder.Replacer, opts *OracleOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "client/cli/query.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}
		template := `cmd.AddCommand(Cmd%[1]vResult())
	cmd.AddCommand(CmdLast%[1]vID())`
		snippet := fmt.Sprintf(template, opts.QueryName.UpperCamel)
		content := f.String()

		if strings.Count(content, Placeholder) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += "\n" + Placeholder
			content = replacer.Replace(f.String(), Placeholder, snippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
			var err error
			content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, snippet, clipper.SelectOptions{
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

func clientCliTxOracleModify(replacer placeholder.Replacer, opts *OracleOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "client/cli/tx.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}
		template := `cmd.AddCommand(CmdRequest%[1]vData())`
		snippet := fmt.Sprintf(template, opts.QueryName.UpperCamel)
		content := f.String()

		if strings.Count(content, Placeholder) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			snippet += "\n" + Placeholder
			content = replacer.Replace(f.String(), Placeholder, snippet)
		} else {
			content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, snippet, clipper.SelectOptions{
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

func codecOracleModify(replacer placeholder.Replacer, opts *OracleOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "types/codec.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Set import if not set yet
		importSnippet := `sdk "github.com/cosmos/cosmos-sdk/types"`
		content, err := clipper.PasteGoImportSnippetAt(path, f.String(), importSnippet)
		if err != nil {
			return err
		}

		// Register the module packet
		templateRegistry := `
	cdc.RegisterConcrete(&Msg%[2]vData{}, "%[1]v/%[2]vData", nil)`
		startOfFunctionSnippet := fmt.Sprintf(templateRegistry, opts.ModuleName, opts.QueryName.UpperCamel)

		if strings.Count(content, Placeholder2) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + Placeholder2
			content = replacer.Replace(content, Placeholder2, startOfFunctionSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
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
		}

		// Register the module packet interface
		templateInterface := `
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&Msg%[1]vData{},
	)`
		startOfFunctionSnippet = fmt.Sprintf(templateInterface, opts.QueryName.UpperCamel)

		if strings.Count(content, Placeholder3) != 0 {
			// To make code generation backwards compatible, we use placeholder mechanism if the code already uses it.
			startOfFunctionSnippet += "\n" + Placeholder3
			content = replacer.Replace(content, Placeholder3, startOfFunctionSnippet)
		} else {
			// And for newer codebase, we use clipper mechanism.
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
		}
		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func packetHandlerOracleModify(replacer placeholder.Replacer, opts *OracleOptions) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "oracle.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// Register the module packet
		templateRecv := `
	case types.%[3]vClientIDKey:
		var %[2]vResult types.%[3]vResult
		if err := obi.Decode(modulePacketData.Result, &%[2]vResult); err != nil {
			ack = channeltypes.NewErrorAcknowledgement(err.Error())
			return ack, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest,
				"cannot decode the %[2]v received packet")
		}
		am.keeper.Set%[3]vResult(ctx, types.OracleRequestID(modulePacketData.RequestID), %[2]vResult)
	
		// TODO: %[3]v oracle data reception logic
%[1]v`
		replacementRegistry := fmt.Sprintf(templateRecv, PlaceholderOracleModuleRecv,
			opts.QueryName.LowerCamel, opts.QueryName.UpperCamel)
		content := replacer.Replace(f.String(), PlaceholderOracleModuleRecv, replacementRegistry)

		// Register the module packet interface
		templateAck := `
	case types.%[3]vClientIDKey:
		var %[2]vData types.%[3]vCallData
		if err = obi.Decode(data.GetCalldata(), &%[2]vData); err != nil {
			return nil, sdkerrors.Wrap(err,
				"cannot decode the %[2]v oracle acknowledgment packet")
		}
		am.keeper.SetLast%[3]vID(ctx, requestID)
		return &sdk.Result{}, nil
%[1]v`
		replacementInterface := fmt.Sprintf(templateAck, PlaceholderOracleModuleAck,
			opts.QueryName.LowerCamel, opts.QueryName.UpperCamel)
		content = replacer.Replace(content, PlaceholderOracleModuleAck, replacementInterface)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
