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
		t.Fatal("invalid new global position", result)
	}
}

func TestGoSelectNewGlobalPositionAfterImports(t *testing.T) {
	result, err := GoSelectNewGlobalPosition("test.go", groupImportGoFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 59 {
		t.Fatal("invalid new global position", result)
	}
}

const withReturnGoFile = `package rets

func withReturn() int {
	a := 5
	return a
}
`

func TestGoSelectBeforeFunctionReturnsPositionWithReturn(t *testing.T) {
	result, err := GoSelectBeforeFunctionReturnsPosition("test.go", withReturnGoFile, SelectOptions{
		"functionName": "withReturn",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 47 {
		t.Fatal("invalid new position before return", result)
	}
}

const noReturnGoFile = `package rets

func withNoReturn() int {
	a := 5
	fmt.Println(a)
}
`

func TestGoSelectBeforeFunctionReturnsPositionWithNoReturn(t *testing.T) {
	result, err := GoSelectBeforeFunctionReturnsPosition("test.go", noReturnGoFile, SelectOptions{
		"functionName": "withNoReturn",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 63 {
		t.Fatal("invalid new position at function end", result)
	}
}

func TestGoSelectStartOfFunctionPosition(t *testing.T) {
	result, err := GoSelectStartOfFunctionPosition("test.go", noImportGoFile, SelectOptions{
		"functionName": "main",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 27 {
		t.Fatal("invalid function block start position", result)
	}
}

const functionCallReturnSingleLineFile = `package test

func returnsValue() int {
	return call(arg0, arg1)
}
`

func TestGoSelectReturningFunctionCallPositionInSingleLine(t *testing.T) {
	result, err := GoSelectReturningFunctionCallNewArgumentPosition("test.go", functionCallReturnSingleLineFile, SelectOptions{
		"functionName": "returnsValue",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 63 {
		t.Fatal("invalid function call new argument position", result)
	}

	if result.Data.(GoReturningFunctionCallNewArgumentPositionData).HasTrailingComma {
		t.Fatal("invalid data after position selection", result)
	}
}

const functionCallReturnMultiLineFile = `package test

func returnsValue() int {
	return call(
		arg0,
		arg1,
	)
}
`

func TestGoSelectReturningFunctionCallPositionInMultiLine(t *testing.T) {
	result, err := GoSelectReturningFunctionCallNewArgumentPosition("test.go", functionCallReturnMultiLineFile, SelectOptions{
		"functionName": "returnsValue",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 71 {
		t.Fatal("invalid function call new argument position", result)
	}

	if !result.Data.(GoReturningFunctionCallNewArgumentPositionData).HasTrailingComma {
		t.Fatal("invalid data after position selection", result)
	}
}

const functionCallReturnNoArgumentsFile = `package test

func returnsValue() int {
	return call()
}
`

func TestGoSelectReturningFunctionCallPositionWhenNoArguments(t *testing.T) {
	result, err := GoSelectReturningFunctionCallNewArgumentPosition("test.go", functionCallReturnNoArgumentsFile, SelectOptions{
		"functionName": "returnsValue",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.OffsetPosition != 53 {
		t.Fatal("invalid function call new argument position", result)
	}

	if result.Data.(GoReturningFunctionCallNewArgumentPositionData).HasTrailingComma {
		t.Fatal("invalid data after position selection", result)
	}
}
