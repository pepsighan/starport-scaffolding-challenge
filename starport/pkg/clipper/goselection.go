package clipper

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// GoNewImportPositionData stores data collected during a selection of a new import position.
type GoNewImportPositionData struct {
	ShouldAddNewLine bool
	OnlyURLNeeded    bool
}

// GoBeforeFunctionReturnsPositionData stores data collected during a selection of the position before a return.
type GoBeforeFunctionReturnsPositionData struct {
	HasReturn bool
}

// GoReturningFunctionCallNewArgumentPositionData stores data collected during a selection of the position for
// a new argument in a function call which is being returned.
type GoReturningFunctionCallNewArgumentPositionData struct {
	HasTrailingComma bool
}

// GoReturningCompositeNewArgumentPositionData stores data collected during a selection of the position for
// a new argument in a struct which is being returned.
type GoReturningCompositeNewArgumentPositionData struct {
	HasTrailingComma bool
}

// goPositionFinder tries to find a required position during a walk of the Golang AST.
type goPositionFinder func(result *PositionSelectorResult, options SelectOptions, code string) goVisitor

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
		ast.Walk(finder(result, options, code), parsedAST)

		if result.OffsetPosition != NoOffsetPosition {
			// The offset position coming from the finder is 1-indexed. So making it 0-indexed.
			result.OffsetPosition -= 1
		}

		return result, nil
	}
}

// GoSelectNewImportPosition selects a position where in a new import can be added.
var GoSelectNewImportPosition = wrapGoFinder(
	func(result *PositionSelectorResult, options SelectOptions, _ string) goVisitor {
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
	func(result *PositionSelectorResult, options SelectOptions, _ string) goVisitor {
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

// GoSelectBeforeFunctionReturnsPosition selects a position just before the last function return (implicit or explicit).
// This only considers the function return which is at the function return.
var GoSelectBeforeFunctionReturnsPosition = wrapGoFinder(
	func(result *PositionSelectorResult, options SelectOptions, _ string) goVisitor {
		functionName := options["functionName"]

		return func(node ast.Node) bool {
			// Select a position after the package declaration or all the imports.
			if n, ok := node.(*ast.FuncDecl); ok && n.Name.Name == functionName {
				lastItem := n.Body.List[len(n.Body.List)-1]

				switch l := lastItem.(type) {
				case *ast.ReturnStmt:
					// If there is a return, select a position before it.
					result.OffsetPosition = OffsetPosition(l.Pos())
					result.Data = GoBeforeFunctionReturnsPositionData{
						HasReturn: true,
					}
				default:
					// Select the last position as there is no return here.
					result.OffsetPosition = OffsetPosition(lastItem.End())
					result.Data = GoBeforeFunctionReturnsPositionData{
						HasReturn: false,
					}
				}
			}

			return true
		}
	},
)

// GoSelectStartOfFunctionPosition selects a position just after the function block starts.
var GoSelectStartOfFunctionPosition = wrapGoFinder(
	func(result *PositionSelectorResult, options SelectOptions, _ string) goVisitor {
		functionName := options["functionName"]

		return func(node ast.Node) bool {
			if n, ok := node.(*ast.FuncDecl); ok && n.Name.Name == functionName {
				// Select the position after the left brace.
				result.OffsetPosition = OffsetPosition(n.Body.Lbrace + 1)
			}
			return true
		}
	},
)

// GoSelectReturningFunctionCallNewArgumentPosition selects a position for a new argument in a function call that is
// returning a value. This function call must be in the returning statement.
var GoSelectReturningFunctionCallNewArgumentPosition = wrapGoFinder(
	func(result *PositionSelectorResult, options SelectOptions, code string) goVisitor {
		functionName := options["functionName"]

		return func(node ast.Node) bool {
			if n, ok := node.(*ast.FuncDecl); ok && n.Name.Name == functionName {
				lastItem := n.Body.List[len(n.Body.List)-1]

				if l, ok := lastItem.(*ast.ReturnStmt); ok && len(l.Results) == 1 {
					ret := l.Results[0]

					if r, ok := ret.(*ast.CallExpr); ok {
						result.OffsetPosition = OffsetPosition(r.Rparen)
						result.Data = GoReturningFunctionCallNewArgumentPositionData{}

						// Check if the closing parenthesis is preceded by a comma.
						leftPart := []rune(strings.TrimSpace(code[:r.Rparen-1]))
						if leftPart[len(leftPart)-1] == ',' {
							result.Data = GoReturningFunctionCallNewArgumentPositionData{
								HasTrailingComma: true,
							}
						}
					}
				}
			}

			return true
		}
	},
)

// GoSelectReturningCompositeNewArgumentPosition selects a position for a new argument in a struct/map that is being
// returned a value. This function call must be in the returning statement.
var GoSelectReturningCompositeNewArgumentPosition = wrapGoFinder(
	func(result *PositionSelectorResult, options SelectOptions, code string) goVisitor {
		functionName := options["functionName"]

		return func(node ast.Node) bool {
			if n, ok := node.(*ast.FuncDecl); ok && n.Name.Name == functionName {
				lastItem := n.Body.List[len(n.Body.List)-1]

				if l, ok := lastItem.(*ast.ReturnStmt); ok && len(l.Results) == 1 {
					ret := l.Results[0]

					if r, ok := ret.(*ast.CompositeLit); ok {
						result.OffsetPosition = OffsetPosition(r.Rbrace)
						result.Data = GoReturningCompositeNewArgumentPositionData{}

						// Check if the closing brace is preceded by a comma.
						leftPart := []rune(strings.TrimSpace(code[:r.Rbrace-1]))
						if leftPart[len(leftPart)-1] == ',' {
							result.Data = GoReturningCompositeNewArgumentPositionData{
								HasTrailingComma: true,
							}
						}
					}
				}
			}

			return true
		}
	},
)
