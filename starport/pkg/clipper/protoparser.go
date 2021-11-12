package clipper

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

// parseProto parses the content as a protobuf file. Content is a the .proto file
// read as string.
func parseProto(content string) (*ast.FileNode, error) {
	parser := protoparse.Parser{
		// protoparse library requires a file to parse, so we modify its accessor
		// to allow a string to behave as a file.
		Accessor: func(filename string) (io.ReadCloser, error) {
			return ioutil.NopCloser(strings.NewReader(content)), nil
		},
	}

	// It does not matter what filename is passed, it is only going to read the passed content.
	parsed, err := parser.ParseToAST("file.proto")
	if err != nil {
		return nil, err
	}

	return parsed[0], nil
}
