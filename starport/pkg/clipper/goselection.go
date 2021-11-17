package clipper

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// GoNewImportPositionData stores data collected during a selection of a new import position.
type GoNewImportPositionData struct {
	ShouldAddNewLine bool
	OnlyURLNeeded    bool
}

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

		if result.OffsetPosition != NoOffsetPosition {
			// The offset position coming from the finder is 1-indexed. So making it 0-indexed.
			result.OffsetPosition -= 1
		}

		return result, nil
	}
}

// GoSelectNewImportPosition selects a position where in a new import can be added.
var GoSelectNewImportPosition = wrapGoFinder(
	func(result *PositionSelectorResult, options SelectOptions) goVisitor {
		return func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.File:
				// Adds new import after package declaration: `package name`.
				result.OffsetPosition = OffsetPosition(n.Name.End())
				result.Data = GoNewImportPositionData{
					ShouldAddNewLine: true,
				}
			case *ast.GenDecl:
				if n.Tok == token.IMPORT {
					// Adds new import after the last import URL.
					result.OffsetPosition = OffsetPosition(n.Specs[len(n.Specs)-1].End())
					result.Data = GoNewImportPositionData{}

					// If this is a group import, only URL is needed for the new one.
					if n.Lparen != token.NoPos {
						result.Data = GoNewImportPositionData{
							OnlyURLNeeded: true,
						}
					}
				}
			}

			return true
		}
	},
)

// GoSelectNewGlobalPosition selects a position a new variable declaration, function or anything global can be
// added.
var GoSelectNewGlobalPosition = wrapGoFinder(
	func(result *PositionSelectorResult, options SelectOptions) goVisitor {
		return func(node ast.Node) bool {
			// Select a position after the package declaration or all the imports.
			switch n := node.(type) {
			case *ast.File:
				result.OffsetPosition = OffsetPosition(n.Name.End())
			case *ast.GenDecl:
				if n.Tok == token.IMPORT {
					result.OffsetPosition = OffsetPosition(n.End())
				}
			}

			return true
		}
	},
)
