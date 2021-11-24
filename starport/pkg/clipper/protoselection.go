package clipper

import (
	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

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

// protoPositionFinder tries to find a required position during a walk of the protobuf AST.
type protoPositionFinder func(result *PositionSelectorResult, options SelectOptions, offsetMap lineOffsetMap) ast.VisitFunc

// wrapProtoFinder creates a selector out of each finder.
func wrapProtoFinder(find protoPositionFinder) *PositionSelector {
	positionSelectorID += 1

	return &PositionSelector{
		id: positionSelectorID,
		call: func(path, code string, options SelectOptions) (*PositionSelectorResult, error) {
			parsedAST, err := parseProto(path, code)
			if err != nil {
				return nil, err
			}

			offsetMap, err := lineOffsetMapOfFile(code)
			if err != nil {
				return nil, err
			}

			if options == nil {
				options = SelectOptions{}
			}

			result := &PositionSelectorResult{
				OffsetPosition: NoOffsetPosition,
			}
			ast.Walk(parsedAST, find(result, options, offsetMap))
			return result, nil
		},
	}
}

// ProtoSelectNewImportPosition selects a position for where a new import can be added. For example: right after
// existing imports or the package declaration.
var ProtoSelectNewImportPosition = wrapProtoFinder(
	func(result *PositionSelectorResult, options SelectOptions, offsetMap lineOffsetMap) ast.VisitFunc {
		return func(node ast.Node) (bool, ast.VisitFunc) {
			// Find the last item position. New import will be appended to the last import item
			// if it exists.
			switch n := node.(type) {
			case *ast.PackageNode:
				result.OffsetPosition = offsetForProtoSourcePos(offsetMap, n.End())
				// The incoming imports require an extra new line after package declaration.
				result.Data = ProtoNewImportPositionData{
					ShouldAddNewLine: true,
				}
			case *ast.ImportNode:
				result.OffsetPosition = offsetForProtoSourcePos(offsetMap, n.End())
				result.Data = ProtoNewImportPositionData{
					ShouldAddNewLine: false,
				}
			}

			return true, nil
		}
	},
)

// ProtoSelectNewMessageFieldPosition selects a position for where a new field in a message can be added.
var ProtoSelectNewMessageFieldPosition = wrapProtoFinder(
	func(result *PositionSelectorResult, options SelectOptions, offsetMap lineOffsetMap) ast.VisitFunc {
		return func(node ast.Node) (bool, ast.VisitFunc) {
			if n, ok := node.(*ast.MessageNode); ok {
				if n.Name.Val == options["name"] {
					// If the message's name matches then we are on the correct one.
					result.OffsetPosition = offsetForProtoSourcePos(offsetMap, n.CloseBrace.Start())

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
var ProtoSelectNewServiceMethodPosition = wrapProtoFinder(
	func(result *PositionSelectorResult, options SelectOptions, offsetMap lineOffsetMap) ast.VisitFunc {
		return func(node ast.Node) (bool, ast.VisitFunc) {
			if n, ok := node.(*ast.ServiceNode); ok {
				if n.Name.Val == options["name"] {
					// If the message's name matches then we are on the correct one.
					result.OffsetPosition = offsetForProtoSourcePos(offsetMap, n.CloseBrace.Start())
				}
			}

			return true, nil
		}
	},
)

// ProtoSelectNewOneOfFieldPosition selects a position for where a new oneof field can be added which is
// itself present within a message.
var ProtoSelectNewOneOfFieldPosition = wrapProtoFinder(
	func(result *PositionSelectorResult, options SelectOptions, offsetMap lineOffsetMap) ast.VisitFunc {
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
					result.OffsetPosition = offsetForProtoSourcePos(offsetMap, n.CloseBrace.Start())

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
var ProtoSelectLastPosition = &PositionSelector{
	id: func() int {
		positionSelectorID += 1
		return positionSelectorID
	}(),
	call: func(path, code string, options SelectOptions) (*PositionSelectorResult, error) {
		parsedAST, err := parseProto(path, code)
		if err != nil {
			return nil, err
		}

		offsetMap, err := lineOffsetMapOfFile(code)
		if err != nil {
			return nil, err
		}

		result := PositionSelectorResult{
			OffsetPosition: offsetForProtoSourcePos(offsetMap, parsedAST.End()),
		}
		return &result, nil
	},
}
