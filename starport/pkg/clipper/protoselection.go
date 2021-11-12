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
