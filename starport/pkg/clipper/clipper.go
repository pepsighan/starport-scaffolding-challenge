package clipper

import (
	"fmt"
)

// PasteProtoCodeAt pastes a proto snippet at the location pointed by the selector and returns a new code.
func PasteProtoCodeAt(code string, selector ProtoPositionSelector, options SelectOptions, snippet string) (string, error) {
	result, err := selector(code, options)
	if err != nil {
		return "", err
	}

	if result.SourcePosition == nil {
		// TODO: Return proper error type.
		return "", fmt.Errorf("did not find any place to paste the generated code to")
	}

	offsetPosition, err := offsetForProtoSourcePos(code, result.SourcePosition)
	if err != nil {
		return "", err
	}

	newContent := code[:offsetPosition] + snippet + code[offsetPosition:]
	return newContent, nil
}
