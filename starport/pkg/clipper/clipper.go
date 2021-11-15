package clipper

import (
	"fmt"
)

// SnippetGenerator generates a snippet to be pasted based on the given data.
type SnippetGenerator func(data interface{}) string

// PasteProtoSnippetAt pastes a proto snippet at the location pointed by the selector and returns a new code.
func PasteProtoSnippetAt(code string, selector ProtoPositionSelector, options SelectOptions, snippet string) (string, error) {
	return PasteGeneratedProtoSnippetAt(code, selector, options, func(_ interface{}) string {
		return snippet
	})
}

// PasteGeneratedProtoSnippetAt pastes a generated proto snippet at the location pointed by the selector and returns
// a new code.
func PasteGeneratedProtoSnippetAt(
	code string, selector ProtoPositionSelector, options SelectOptions, generator SnippetGenerator,
) (string, error) {
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

	snippet := generator(result.Data)
	newContent := code[:offsetPosition] + snippet + code[offsetPosition:]
	return newContent, nil
}

// PasteProtoImportSnippetAt pastes an import snippet at the start of the file while making sure that there
// is an empty space between package declaration and import.
func PasteProtoImportSnippetAt(code string, snippet string) (string, error) {
	return PasteGeneratedProtoSnippetAt(
		code,
		ProtoSelectNewImportPosition,
		nil,
		func(data interface{}) string {
			shouldAddNewLine := data.(ProtoNewImportPositionData).ShouldAddNewLine
			if shouldAddNewLine {
				return fmt.Sprintf("\n%v", snippet)
			}
			return snippet
		},
	)
}
