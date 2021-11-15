package clipper

import "testing"

func TestGoSelectNewImportLocationAfterNoImports(t *testing.T) {
	result, err := GoSelectNewImportLocation("test.go", `package test

func main() {}
`, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 13 {
		t.Fatal("invalid new import position", result)
	}
}

func TestGoSelectNewImportLocationAfterImports(t *testing.T) {
	result, err := GoSelectNewImportLocation("test.go", `package test

import "testing"

func main() {}
`, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 31 {
		t.Fatal("invalid new import position", result)
	}
}

func TestGoSelectNewImportLocationAfterImportsGroup(t *testing.T) {
	result, err := GoSelectNewImportLocation("test.go", `package test

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
}
