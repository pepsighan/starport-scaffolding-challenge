package clipper

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

// parseProto parses the code as a protobuf file.
func parseProto(code string) (*ast.FileNode, error) {
	parser := protoparse.Parser{
		// protoparse library requires a file to parse, so we modify its accessor
		// to allow a string to behave as a file.
		Accessor: func(filename string) (io.ReadCloser, error) {
			return ioutil.NopCloser(strings.NewReader(code)), nil
		},
	}

	// It does not matter what filename is passed, it is only going to read the passed code.
	parsed, err := parser.ParseToAST("file.proto")
	if err != nil {
		return nil, err
	}

	return parsed[0], nil
}
