package clipper

import (
	"testing"
)

func TestAddingProtoImport(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.proto",
		genesisProtoFile,
		ProtoSelectNewImportPosition,
		nil,
		"\nimport \"google/api/annotations.proto\";",
	)

	if err != nil {
		t.Fatal(err)
	}

	correct := `syntax = "proto3";
package cosmonaut.mars.mars;
import "google/api/annotations.proto";

option go_package = "github.com/cosmonaut/mars/x/mars/types";

// GenesisState defines the mars module's genesis state.
message GenesisState {

}`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingImportAfterImports(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.proto",
		queryProtoFile,
		ProtoSelectNewImportPosition,
		nil,
		"\nimport \"google/api/types.proto\";",
	)

	if err != nil {
		t.Fatal(err)
	}

	correct := `syntax = "proto3";
package cosmonaut.mars.mars;

import "google/api/annotations.proto";
import "cosmos/base/query/v1beta1/pagination.proto";
import "google/api/types.proto";

option go_package = "github.com/cosmonaut/mars/x/mars/types";

// Query defines the gRPC query service.
service Query {

}`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingMessageField(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.proto",
		genesisProtoFile,
		ProtoSelectNewMessageFieldPosition,
		SelectOptions{
			"name": "GenesisState",
		},
		"  required string query = 1;\n",
	)

	if err != nil {
		t.Fatal(err)
	}

	correct := `syntax = "proto3";
package cosmonaut.mars.mars;

option go_package = "github.com/cosmonaut/mars/x/mars/types";

// GenesisState defines the mars module's genesis state.
message GenesisState {

  required string query = 1;
}`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingServiceMethod(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.proto",
		queryProtoFile,
		ProtoSelectNewServiceMethodPosition,
		SelectOptions{
			"name": "Query",
		},
		"  rpc Search(SearchRequest) returns (SearchResponse);\n",
	)

	if err != nil {
		t.Fatal(err)
	}

	correct := `syntax = "proto3";
package cosmonaut.mars.mars;

import "google/api/annotations.proto";
import "cosmos/base/query/v1beta1/pagination.proto";

option go_package = "github.com/cosmonaut/mars/x/mars/types";

// Query defines the gRPC query service.
service Query {

  rpc Search(SearchRequest) returns (SearchResponse);
}`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingOneOfField(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.proto",
		packetProtoFile,
		ProtoSelectNewOneOfFieldPosition,
		SelectOptions{
			"messageName": "MarsPacketData",
			"oneOfName":   "packet",
		},
		"  string name = 2;\n  ",
	)

	if err != nil {
		t.Fatal(err)
	}

	correct := `syntax = "proto3";
package cosmonaut.mars.mars;

option go_package = "github.com/cosmonaut/mars/x/mars/types";

message MarsPacketData {
  oneof packet {
    NoData noData = 1;
    string name = 2;
  }
}

message NoData {

}`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingNewImportAfterPackage(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.go",
		noImportGoFile,
		GoSelectNewImportPosition,
		nil,
		"\nimport \"fmt\"",
	)
	if err != nil {
		t.Fatal(err)
	}

	correct := `package test
import "fmt"

func main() {}
`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingNewImportAfterImportsGroup(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.go",
		groupImportGoFile,
		GoSelectNewImportPosition,
		nil,
		"\n\t\"fmt\"",
	)
	if err != nil {
		t.Fatal(err)
	}

	correct := `package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"fmt"
)

func main() {}
`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingConstDeclarationGlobally(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.go",
		noImportGoFile,
		GoSelectNewGlobalPosition,
		nil,
		"\n\nconst a = 5",
	)
	if err != nil {
		t.Fatal(err)
	}

	correct := `package test

const a = 5

func main() {}
`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingConstDeclarationGloballyAfterImports(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.go",
		groupImportGoFile,
		GoSelectNewGlobalPosition,
		nil,
		"\n\nconst a = 5",
	)
	if err != nil {
		t.Fatal(err)
	}

	correct := `package test

import (
	"go/ast"
	"go/parser"
	"go/token"
)

const a = 5

func main() {}
`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingBeforeExplicitReturnInFunction(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.go",
		withReturnGoFile,
		GoSelectBeforeFunctionReturnsPosition,
		SelectOptions{
			"functionName": "withReturn",
		},
		"b := 5\n\t",
	)
	if err != nil {
		t.Fatal(err)
	}

	correct := `package rets

func withReturn() int {
	a := 5
	b := 5
	return a
}
`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingBeforeFunctionEnds(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.go",
		noReturnGoFile,
		GoSelectBeforeFunctionReturnsPosition,
		SelectOptions{
			"functionName": "withNoReturn",
		},
		"\n\tb := 5",
	)
	if err != nil {
		t.Fatal(err)
	}

	correct := `package rets

func withNoReturn() int {
	a := 5
	fmt.Println(a)
	b := 5
}
`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}

func TestAddingFirstInTheFunction(t *testing.T) {
	generated, err := New().PasteCodeSnippetAt(
		"test.go",
		noReturnGoFile,
		GoSelectStartOfFunctionPosition,
		SelectOptions{
			"functionName": "withNoReturn",
		},
		"\n\tb := 5",
	)
	if err != nil {
		t.Fatal(err)
	}

	correct := `package rets

func withNoReturn() int {
	b := 5
	a := 5
	fmt.Println(a)
}
`

	if generated != correct {
		t.Fatal("incorrect generation: \n", generated)
	}
}
