package clipper

import "testing"

func TestParsingProtoString(t *testing.T) {
	parsed, err := parseProto(`syntax = "proto3";
package cosmonaut.mars.mars;
option go_package = "github.com/cosmonaut/mars/x/mars/types";
// GenesisState defines the mars module's genesis state.
message GenesisState {
}
`)
	if err != nil {
		t.Fatal("could not parse proto string")
	}

	if parsed.End().Line != 9 || parsed.End().Col != 2 {
		t.Fatal("failed to parse proto string correctly")
	}
}
