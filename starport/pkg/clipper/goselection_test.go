package clipper

import "testing"

const noImportGoFile = `package test

func main() {}
`

func TestGoSelectNewImportPositionAfterNoImports(t *testing.T) {
	result, err := GoSelectNewImportPosition("test.go", noImportGoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 13 {
		t.Fatal("invalid new import position", result)
	}

	data := result.Data.(GoNewImportPositionData)
	if data.OnlyURLNeeded || !data.ShouldAddNewLine {
		t.Fatal("invalid new import data", result)
	}
}

const oneImportGoFile = `package test

import "testing"

func main() {}
`

func TestGoSelectNewImportPositionAfterImports(t *testing.T) {
	result, err := GoSelectNewImportPosition("test.go", oneImportGoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 31 {
		t.Fatal("invalid new import position", result)
	}

	data := result.Data.(GoNewImportPositionData)
	if data.OnlyURLNeeded || data.ShouldAddNewLine {
		t.Fatal("invalid new import data", result)
	}
}

const groupImportGoFile = `package test

import (
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {}
`

func TestGoSelectNewImportPositionAfterImportsGroup(t *testing.T) {
	result, err := GoSelectNewImportPosition("test.go", groupImportGoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 58 {
		t.Fatal("invalid new import position", result)
	}

	data := result.Data.(GoNewImportPositionData)
	if !data.OnlyURLNeeded || data.ShouldAddNewLine {
		t.Fatal("invalid new import data", result)
	}
}
