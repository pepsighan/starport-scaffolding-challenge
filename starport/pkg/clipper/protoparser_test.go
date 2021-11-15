package clipper

import "testing"

func TestParsingProtoString(t *testing.T) {
	parsed, err := parseProto("test.proto", genesisProtoFile)
	if err != nil {
		t.Fatal("could not parse proto string")
	}

	if parsed.End().Line != 9 || parsed.End().Col != 2 {
		t.Fatal("failed to parse proto string correctly")
	}
}
