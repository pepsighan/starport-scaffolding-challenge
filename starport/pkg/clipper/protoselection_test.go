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
	result, err := ProtoSelectNewImportPosition("test.proto", genesisProtoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition == NoOffsetPosition {
		t.Fatal("did not find import location")
	}

	if result.OffsetPosition != 47 {
		t.Fatal("wrong result found", result)
	}

	if result.Data.(ProtoNewImportPositionData).ShouldAddNewLine != true {
		t.Fatal("wrong result found", result)
	}
}

func TestProtoSelectNewImportPositionForQuery(t *testing.T) {
	result, err := ProtoSelectNewImportPosition("test.proto", queryProtoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition == NoOffsetPosition {
		t.Fatal("did not find import location")
	}

	if result.OffsetPosition != 140 {
		t.Fatal("wrong result found", result)
	}

	if result.Data.(ProtoNewImportPositionData).ShouldAddNewLine != false {
		t.Fatal("wrong result found", result)
	}
}

func TestProtoSelectNewMessageFieldPosition(t *testing.T) {
	result, err := ProtoSelectNewMessageFieldPosition("test.proto", genesisProtoFile, SelectOptions{
		"name": "GenesisState",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition == NoOffsetPosition {
		t.Fatal("did not find message")
	}

	if result.OffsetPosition != 193 {
		t.Fatal("wrong result found", result)
	}
}

func TestProtoSelectNewServiceMethodPosition(t *testing.T) {
	result, err := ProtoSelectNewServiceMethodPosition("test.proto", queryProtoFile, SelectOptions{
		"name": "Query",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition == NoOffsetPosition {
		t.Fatal("did not find message")
	}

	if result.OffsetPosition != 263 {
		t.Fatal("wrong result found", result)
	}
}

func TestDoNotFindNewOneOfFieldPosition(t *testing.T) {
	result, err := ProtoSelectNewOneOfFieldPosition("test.proto", packetProtoFile, SelectOptions{
		"messageName": "NoData",
		"oneOfName":   "packet",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != NoOffsetPosition {
		t.Fatal("wrong result found", result)
	}
}

func TestProtoSelectLastPosition(t *testing.T) {
	result, err := ProtoSelectLastPosition("test.proto", queryProtoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition == NoOffsetPosition {
		t.Fatal("did not find message")
	}

	if result.OffsetPosition != 264 {
		t.Fatal("wrong result found", result)
	}
}
