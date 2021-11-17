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

	if result.OffsetPosition != 12 {
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

	if result.OffsetPosition != 30 {
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

	if result.OffsetPosition != 57 {
		t.Fatal("invalid new import position", result)
	}

	data := result.Data.(GoNewImportPositionData)
	if !data.OnlyURLNeeded || data.ShouldAddNewLine {
		t.Fatal("invalid new import data", result)
	}
}

func TestGoSelectNewGlobalPositionAfterNoImports(t *testing.T) {
	result, err := GoSelectNewGlobalPosition("test.go", noImportGoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 12 {
		t.Fatal("invalid new import position", result)
	}
}

func TestGoSelectNewGlobalPositionAfterImports(t *testing.T) {
	result, err := GoSelectNewGlobalPosition("test.go", groupImportGoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 59 {
		t.Fatal("invalid new import position", result)
	}
}
