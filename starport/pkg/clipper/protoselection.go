package clipper

import "github.com/jhump/protoreflect/desc/protoparse/ast"

// SelectOptions are the options needed to configure a particular selector.
type SelectOptions map[string]string

// ProtoPositionSelectorResult contains position in code after a successful selection.
type ProtoPositionSelectorResult struct {
	SourcePosition *ast.SourcePos
}

// ProtoPositionSelector is a configurable selector which can select a position in code.
type ProtoPositionSelector func(code string, options SelectOptions) (*ProtoPositionSelectorResult, error)

// protoPositionFinder tries to find a required position during a walk of the protobuf AST.
type protoPositionFinder func(sourcePos *ProtoPositionSelectorResult, options SelectOptions) ast.VisitFunc

// wrapFinder creates a selector out of each finder.
func wrapFinder(find protoPositionFinder) ProtoPositionSelector {
	return func(content string, options SelectOptions) (*ProtoPositionSelectorResult, error) {
		parsedAST, err := parseProto(content)
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
