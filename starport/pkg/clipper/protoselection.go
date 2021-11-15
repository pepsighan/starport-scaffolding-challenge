package clipper

import (
	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

// SelectOptions are the options needed to configure a particular selector.
type SelectOptions map[string]string

// ProtoPositionSelectorResult contains position in code after a successful selection.
type ProtoPositionSelectorResult struct {
	SourcePosition *ast.SourcePos
	// Any additional piece of data collected during a selection.
	Data interface{}
}

// ProtoNewImportPositionData stores data collected during a selection of new import position.
type ProtoNewImportPositionData struct {
	ShouldAddNewLine bool
}

// ProtoNewMessageFieldPositionData stores data collected during a selection of a new message field position.
type ProtoNewMessageFieldPositionData struct {
	HighestFieldNumber uint64
}

// ProtoNewOneOfFieldPositionData stores data collected during a selection of new oneof field position.
type ProtoNewOneOfFieldPositionData struct {
	HighestFieldNumber uint64
}

// ProtoPositionSelector is a configurable selector which can select a position in code.
type ProtoPositionSelector func(path, code string, options SelectOptions) (*ProtoPositionSelectorResult, error)

// protoPositionFinder tries to find a required position during a walk of the protobuf AST.
type protoPositionFinder func(result *ProtoPositionSelectorResult, options SelectOptions) ast.VisitFunc

// wrapFinder creates a selector out of each finder.
func wrapFinder(find protoPositionFinder) ProtoPositionSelector {
	return func(path, code string, options SelectOptions) (*ProtoPositionSelectorResult, error) {
		parsedAST, err := parseProto(path, code)
		if err != nil {
			return nil, err
		}

		if options == nil {
			options = SelectOptions{}
		}

		result := &ProtoPositionSelectorResult{}
		ast.Walk(parsedAST, find(result, options))
		return result, nil
	}
}

// ProtoSelectNewImportPosition selects a position for where a new import can be added. For example: right after
// existing imports or the package declaration.
var ProtoSelectNewImportPosition = wrapFinder(
	func(result *ProtoPositionSelectorResult, options SelectOptions) ast.VisitFunc {
		return func(node ast.Node) (bool, ast.VisitFunc) {
			// Find the last item position. New import will be appended to the last import item
			// if it exists.
			switch n := node.(type) {
			case *ast.PackageNode:
				result.SourcePosition = n.End()
				// The incoming imports require an extra new line after package declaration.
				result.Data = ProtoNewImportPositionData{
					ShouldAddNewLine: true,
				}
			case *ast.ImportNode:
				result.SourcePosition = n.End()
				result.Data = ProtoNewImportPositionData{
					ShouldAddNewLine: false,
				}
			}

			return true, nil
		}
	},
)

// ProtoSelectNewMessageFieldPosition selects a position for where a new field in a message can be added.
var ProtoSelectNewMessageFieldPosition = wrapFinder(
	func(result *ProtoPositionSelectorResult, options SelectOptions) ast.VisitFunc {
		return func(node ast.Node) (bool, ast.VisitFunc) {
			if n, ok := node.(*ast.MessageNode); ok {
				if n.Name.Val == options["name"] {
					// If the message's name matches then we are on the correct one.
					result.SourcePosition = n.CloseBrace.Start()

					// Get the highest field number so that new additions can have the next value.
					data := ProtoNewMessageFieldPositionData{
						HighestFieldNumber: 0,
					}
					for _, decl := range n.Decls {
						switch d := decl.(type) {
						case *ast.FieldNode:
							if d.Tag.Val > data.HighestFieldNumber {
								data.HighestFieldNumber = d.Tag.Val
							}
						}
					}
					result.Data = data
				}
			}

			return true, nil
		}
	},
)

// ProtoSelectNewServiceMethodPosition selects a position for where a new method in a service can be added.
var ProtoSelectNewServiceMethodPosition = wrapFinder(
	func(result *ProtoPositionSelectorResult, options SelectOptions) ast.VisitFunc {
		return func(node ast.Node) (bool, ast.VisitFunc) {
			if n, ok := node.(*ast.ServiceNode); ok {
				if n.Name.Val == options["name"] {
					// If the message's name matches then we are on the correct one.
					result.SourcePosition = n.CloseBrace.Start()
				}
			}

			return true, nil
		}
	},
)

// ProtoSelectNewOneOfFieldPosition selects a position for where a new oneof field can be added which is
// itself present within a message.
var ProtoSelectNewOneOfFieldPosition = wrapFinder(
	func(result *ProtoPositionSelectorResult, options SelectOptions) ast.VisitFunc {
		initial := false
		isMsgFound := &initial

		return func(node ast.Node) (bool, ast.VisitFunc) {
			switch n := node.(type) {
			case *ast.MessageNode:
				if n.Name.Val == options["messageName"] {
					// Only search within the message node that we require.
					*isMsgFound = true
				}
			case *ast.OneOfNode:
				if *isMsgFound && n.Name.Val == options["oneOfName"] {
					// If the oneof type's name matches then this is it. Select the position just before the ending
					// brace.
					result.SourcePosition = n.CloseBrace.Start()

					// Get the highest field number so that new additions can have the next value.
					data := ProtoNewOneOfFieldPositionData{
						HighestFieldNumber: 0,
					}
					for _, decl := range n.Decls {
						switch d := decl.(type) {
						case *ast.FieldNode:
							if d.Tag.Val > data.HighestFieldNumber {
								data.HighestFieldNumber = d.Tag.Val
							}
						}
					}
					result.Data = data
				}
			}

			return true, nil
		}
	},
)

// ProtoSelectLastPosition selects the last position within the code.
func ProtoSelectLastPosition(path, code string, _ SelectOptions) (*ProtoPositionSelectorResult, error) {
	parsedAST, err := parseProto(path, code)
	if err != nil {
		return nil, err
	}

	return &ProtoPositionSelectorResult{SourcePosition: parsedAST.End()}, nil
}
