package clipper

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

// parseProto parses the code as a protobuf file. The path is the file path for the code as this provides
// more context in errors.
func parseProto(path string, code string) (*ast.FileNode, error) {
	parser := protoparse.Parser{
		// protoparse library requires a file to parse, so we modify its accessor
		// to allow a string to behave as a file.
		Accessor: func(filename string) (io.ReadCloser, error) {
			return ioutil.NopCloser(strings.NewReader(code)), nil
		},
	}

	// It won't read from the path because of above.
	parsed, err := parser.ParseToAST(path)
	if err != nil {
		return nil, err
	}

	return parsed[0], nil
}
