package clipper

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// goPositionFinder tries to find a required position during a walk of the Golang AST.
type goPositionFinder func(result *PositionSelectorResult, options SelectOptions) goVisitor

// goVisitor visits each node in the Golang AST tree until it returns true.
type goVisitor func(node ast.Node) bool

// Visit implements the Visitor interface to walk the Golang AST.
func (f goVisitor) Visit(node ast.Node) ast.Visitor {
	if f(node) {
		return f
	}
	return nil
}

// wrapGoFinder creates a selector out of each finder.
func wrapGoFinder(finder goPositionFinder) PositionSelector {
	return func(path, code string, options SelectOptions) (*PositionSelectorResult, error) {
		parsedAST, err := parser.ParseFile(token.NewFileSet(), path, []byte(code), 0)
		if err != nil {
			return nil, err
		}

		if options == nil {
			options = SelectOptions{}
		}

		result := &PositionSelectorResult{
			OffsetPosition: NoOffsetPosition,
		}
		ast.Walk(finder(result, options), parsedAST)
		return result, nil
	}
}

// GoSelectNewImportLocation selects a position where in a new import can be added.
var GoSelectNewImportLocation = wrapGoFinder(func(result *PositionSelectorResult, options SelectOptions) goVisitor {
	return func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.File:
			// Adds new import after package declaration.
			result.OffsetPosition = OffsetPosition(n.Name.End())
		case *ast.ImportSpec:
			// Adds new import after this one.
			result.OffsetPosition = OffsetPosition(n.End())
		case *ast.GenDecl:
			if n.Tok == token.IMPORT {
				// Adds new import after the last import URL.
				result.OffsetPosition = OffsetPosition(n.Specs[0].End())
			}
		}

		return true
	}
})
