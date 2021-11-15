package clipper

import (
	"testing"

	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

func TestLineOffsetMap(t *testing.T) {
	offsetMap, err := lineOffsetMapOfFile(genesisProtoFile)
	if err != nil {
		t.Fatal(err)
	}

	if offsetMap == nil {
		t.Fatal("cannot be nil")
	}

	validOffsetMap := map[int]int{
		1: 18,
		2: 18 + 29,
		3: 18 + 29 + 1,
		4: 18 + 29 + 1 + 62,
		5: 18 + 29 + 1 + 62 + 1,
		6: 18 + 29 + 1 + 62 + 1 + 57,
		7: 18 + 29 + 1 + 62 + 1 + 57 + 23,
		8: 18 + 29 + 1 + 62 + 1 + 57 + 23 + 1,
		9: 18 + 29 + 1 + 62 + 1 + 57 + 23 + 1 + 2,
	}

	for k, v := range validOffsetMap {
		if offsetMap[k] != v {
			t.Errorf("invalid offset map line:%v valid:%v calculated:%v", k, v, offsetMap[k])
		}
	}
}

func TestCalcOffsetPositionForProtoSourcePosition(t *testing.T) {
	offsetMap, err := lineOffsetMapOfFile(genesisProtoFile)
	if err != nil {
		t.Fatal(err)
	}

	offset := offsetForProtoSourcePos(offsetMap, &ast.SourcePos{Line: 5, Col: 5})
	if offset != 115 {
		t.Fatal("wrong offset position calculated", offset)
	}
}
