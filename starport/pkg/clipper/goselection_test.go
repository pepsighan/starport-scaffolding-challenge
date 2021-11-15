package clipper

import "testing"

func TestGoSelectNewImportPositionAfterNoImports(t *testing.T) {
	result, err := GoSelectNewImportPosition("test.go", `package test

func main() {}
`, nil)
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

func TestGoSelectNewImportPositionAfterImports(t *testing.T) {
	result, err := GoSelectNewImportPosition("test.go", `package test

import "testing"

func main() {}
`, nil)
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

func TestGoSelectNewImportPositionAfterImportsGroup(t *testing.T) {
	result, err := GoSelectNewImportPosition("test.go", `package test

import (
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {}
`, nil)
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
