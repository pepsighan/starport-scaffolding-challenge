package clipper

import (
	"fmt"
)

// SnippetGenerator generates a snippet to be pasted based on the given data.
type SnippetGenerator func(data interface{}) string

// PasteCodeSnippetAt pastes a code snippet at the location pointed by the selector and returns a new code. The path
// is only used for context in errors.
func PasteCodeSnippetAt(path, code string, selector PositionSelector, options SelectOptions, snippet string) (string, error) {
	return PasteGeneratedCodeSnippetAt(path, code, selector, options, func(_ interface{}) string {
		return snippet
	})
}

// PasteGeneratedCodeSnippetAt pastes a generated code snippet at the location pointed by the selector and returns
// a new code. The path is only used for context in errors.
func PasteGeneratedCodeSnippetAt(
	path, code string, selector PositionSelector, options SelectOptions, generator SnippetGenerator,
) (string, error) {
	result, err := selector(path, code, options)
	if err != nil {
		return "", err
	}

	if result.OffsetPosition == NoOffsetPosition {
		// TODO: Return proper error type.
		return "", fmt.Errorf("did not find any place to paste the generated code to")
	}

	offsetPosition := result.OffsetPosition
	snippet := generator(result.Data)
	newContent := code[:offsetPosition] + snippet + code[offsetPosition:]
	return newContent, nil
}

// PasteProtoImportSnippetAt pastes an import snippet at the start of the file while making sure that there
// is an empty space between package declaration and import. The path is only used for context in errors.
func PasteProtoImportSnippetAt(path, code string, snippet string) (string, error) {
	return PasteGeneratedCodeSnippetAt(
		path,
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
