package clipper

import "testing"

const genesisProtoFile = `syntax = "proto3";
package cosmonaut.mars.mars;

option go_package = "github.com/cosmonaut/mars/x/mars/types";

// GenesisState defines the mars module's genesis state.
message GenesisState {

}`

const queryProtoFile = `syntax = "proto3";
package cosmonaut.mars.mars;

import "google/api/annotations.proto";
import "cosmos/base/query/v1beta1/pagination.proto";

option go_package = "github.com/cosmonaut/mars/x/mars/types";

// Query defines the gRPC query service.
service Query {

}`

const packetProtoFile = `syntax = "proto3";
package cosmonaut.mars.mars;

option go_package = "github.com/cosmonaut/mars/x/mars/types";

message MarsPacketData {
  oneof packet {
    NoData noData = 1;
  }
}

message NoData {

}`

func TestProtoSelectNewImportPositionForGenesis(t *testing.T) {
	result, err := ProtoSelectNewImportPosition(genesisProtoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.SourcePosition == nil {
		t.Fatal("did not find import location")
	}

	if result.SourcePosition.Line != 2 || result.SourcePosition.Col != 29 {
		t.Fatal("wrong result found", result)
	}

	if result.Data["shouldAddNewLine"] != true {
		t.Fatal("wrong result found", result)
	}
}

func TestProtoSelectNewImportPositionForQuery(t *testing.T) {
	result, err := ProtoSelectNewImportPosition(queryProtoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.SourcePosition == nil {
		t.Fatal("did not find import location")
	}

	if result.SourcePosition.Line != 5 || result.SourcePosition.Col != 53 {
		t.Fatal("wrong result found", result)
	}

	if result.Data["shouldAddNewLine"] != false {
		t.Fatal("wrong result found", result)
	}
}

func TestProtoSelectNewMessageFieldPosition(t *testing.T) {
	result, err := ProtoSelectNewMessageFieldPosition(genesisProtoFile, SelectOptions{
		"name": "GenesisState",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.SourcePosition == nil {
		t.Fatal("did not find message")
	}

	if result.SourcePosition.Line != 9 || result.SourcePosition.Col != 1 {
		t.Fatal("wrong result found", result)
	}
}

func TestProtoSelectNewServiceMethodPosition(t *testing.T) {
	result, err := ProtoSelectNewServiceMethodPosition(queryProtoFile, SelectOptions{
		"name": "Query",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.SourcePosition == nil {
		t.Fatal("did not find message")
	}

	if result.SourcePosition.Line != 12 || result.SourcePosition.Col != 1 {
		t.Fatal("wrong result found", result)
	}
}

func TestDoNotFindNewOneOfFieldPosition(t *testing.T) {
	result, err := ProtoSelectNewOneOfFieldPosition(packetProtoFile, SelectOptions{
		"messageName": "NoData",
		"oneOfName":   "packet",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.SourcePosition != nil {
		t.Fatal("wrong result found", result)
	}
}

func TestProtoSelectLastPosition(t *testing.T) {
	result, err := ProtoSelectLastPosition(queryProtoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.SourcePosition == nil {
		t.Fatal("did not find message")
	}

	if result.SourcePosition.Line != 12 || result.SourcePosition.Col != 2 {
		t.Fatal("wrong result found", result)
	}
}
