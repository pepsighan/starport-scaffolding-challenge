package clipper

import (
	"fmt"

	"github.com/tendermint/starport/starport/pkg/placeholder"
)

// SnippetGenerator generates a snippet to be pasted based on the given data.
type SnippetGenerator func(data interface{}) string

// Clipper can paste new generated code in place by performing code analysis via selectors. It can also for backwards
// compatibilities sake can perform replacements of new code using placeholders.
type Clipper struct {
	placeholder.Tracer
	// Missing clipper selections that is needed for the clipper to work.
	missingSelections       []int
	missingSelectionOptions []SelectOptions
}

// New creates a new clipper.
func New() *Clipper {
	return &Clipper{}
}

// Err if any selections or placeholders were missing during execution.
func (c *Clipper) Err() error {
	err := &ValidationError{tracerError: c.Tracer.Err()}

	if len(c.missingSelections) > 0 {
		for id, sel := range c.missingSelections {
			switch sel {
			case ProtoSelectNewImportPosition.id:
				err.missingSelections = append(err.missingSelections, "cannot find position to add new import")
			case ProtoSelectNewMessageFieldPosition.id:
				err.missingSelections = append(
					err.missingSelections,
					fmt.Sprintf("cannot find message %v", c.missingSelectionOptions[id]["name"]),
				)
			case ProtoSelectNewServiceMethodPosition.id:
				err.missingSelections = append(
					err.missingSelections,
					fmt.Sprintf("cannot find service %v", c.missingSelectionOptions[id]["name"]),
				)
			case ProtoSelectNewOneOfFieldPosition.id:
				err.missingSelections = append(
					err.missingSelections,
					fmt.Sprintf("cannot find message %v with oneof field %v",
						c.missingSelectionOptions[id]["messageName"], c.missingSelectionOptions[id]["oneOfName"]),
				)
			case ProtoSelectLastPosition.id:
				err.missingSelections = append(err.missingSelections, "cannot find last position of file")
			case GoSelectNewImportPosition.id:
				err.missingSelections = append(err.missingSelections, "cannot find position to add new import")
			case GoSelectNewGlobalPosition.id:
				err.missingSelections = append(err.missingSelections, "cannot find position for global declaration")
			case GoSelectBeforeFunctionReturnsPosition.id:
				err.missingSelections = append(
					err.missingSelections,
					fmt.Sprintf("cannot find function %v", c.missingSelectionOptions[id]["functionName"]),
				)
			case GoSelectStartOfFunctionPosition.id:
				err.missingSelections = append(
					err.missingSelections,
					fmt.Sprintf("cannot find function %v", c.missingSelectionOptions[id]["functionName"]),
				)
			case GoSelectReturningFunctionCallNewArgumentPosition.id:
				err.missingSelections = append(
					err.missingSelections,
					fmt.Sprintf("cannot find function %v which is returning value with a function call",
						c.missingSelectionOptions[id]["functionName"]),
				)
			case GoSelectReturningCompositeNewArgumentPosition.id:
				err.missingSelections = append(
					err.missingSelections,
					fmt.Sprintf("cannot find function %v which is returning value with a map/struct call",
						c.missingSelectionOptions[id]["functionName"]),
				)
			case GoSelectStructNewFieldPosition.id:
				err.missingSelections = append(
					err.missingSelections,
					fmt.Sprintf("cannot find struct %v", c.missingSelectionOptions[id]["structName"]),
				)
			}
		}
	}

	return err
}

// PasteCodeSnippetAt pastes a code snippet at the location pointed by the selector and returns a new code. The path
// is only used for context in errors.
func (c *Clipper) PasteCodeSnippetAt(
	path, code string, selector *PositionSelector, options SelectOptions, snippet string,
) (string, error) {
	return c.PasteGeneratedCodeSnippetAt(path, code, selector, options, func(_ interface{}) string {
		return snippet
	})
}

// PasteGeneratedCodeSnippetAt pastes a generated code snippet at the location pointed by the selector and returns
// a new code. The path is only used for context in errors.
func (c *Clipper) PasteGeneratedCodeSnippetAt(
	path, code string, selector *PositionSelector, options SelectOptions, generator SnippetGenerator,
) (string, error) {
	result, err := selector.call(path, code, options)
	if err != nil {
		return "", err
	}

	if result.OffsetPosition == NoOffsetPosition {
		return "", ErrNoPositionToGenerateCodeFound
	}

	offsetPosition := result.OffsetPosition
	snippet := generator(result.Data)
	newContent := code[:offsetPosition] + snippet + code[offsetPosition:]
	return newContent, nil
}

// PasteProtoImportSnippetAt pastes an import snippet at the start of the file while making sure that there
// is an empty space between package declaration and import. The path is only used for context in errors.
func (c *Clipper) PasteProtoImportSnippetAt(path, code string, snippet string) (string, error) {
	return c.PasteGeneratedCodeSnippetAt(
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

// PasteGoBeforeReturnSnippetAt pastes a Golang snippet right before a function returns at the end of the function
// block.
func (c *Clipper) PasteGoBeforeReturnSnippetAt(
	path, code string, snippet string, options SelectOptions,
) (string, error) {
	return c.PasteGeneratedCodeSnippetAt(
		path,
		code,
		GoSelectBeforeFunctionReturnsPosition,
		options,
		func(data interface{}) string {
			hasReturn := data.(GoBeforeFunctionReturnsPositionData).HasReturn
			if hasReturn {
				return fmt.Sprintf("%v\n\t", snippet)
			}
			return fmt.Sprintf("\n\t%v", snippet)
		},
	)
}

// PasteGoImportSnippetAt pastes a Golang import snippet at the import site.
func (c *Clipper) PasteGoImportSnippetAt(path, code string, snippet string) (string, error) {
	return c.PasteGeneratedCodeSnippetAt(
		path,
		code,
		GoSelectNewImportPosition,
		nil,
		func(data interface{}) string {
			importData := data.(GoNewImportPositionData)
			if importData.OnlyURLNeeded {
				return fmt.Sprintf("\n%v", snippet)
			}
			return fmt.Sprintf("\nimport (\n\t%v\n)", snippet)
		},
	)
}

// PasteGoReturningFunctionNewArgumentSnippetAt pastes argument for a returning function in a function.
func (c *Clipper) PasteGoReturningFunctionNewArgumentSnippetAt(
	path, code string, snippet string, options SelectOptions,
) (string, error) {
	return c.PasteGeneratedCodeSnippetAt(
		path,
		code,
		GoSelectReturningFunctionCallNewArgumentPosition,
		options,
		func(data interface{}) string {
			hasTrailingComma := data.(GoReturningFunctionCallNewArgumentPositionData).HasTrailingComma
			if hasTrailingComma {
				return fmt.Sprintf("\t%v,\n\t", snippet)
			}
			return fmt.Sprintf(", %v,", snippet)
		},
	)
}

// PasteGoReturningCompositeNewArgumentSnippetAt pastes argument for a returning struct in a function.
func (c *Clipper) PasteGoReturningCompositeNewArgumentSnippetAt(
	path, code string, snippet string, options SelectOptions,
) (string, error) {
	return c.PasteGeneratedCodeSnippetAt(
		path,
		code,
		GoSelectReturningCompositeNewArgumentPosition,
		options,
		func(data interface{}) string {
			hasTrailingComma := data.(GoReturningCompositeNewArgumentPositionData).HasTrailingComma
			if hasTrailingComma {
				return fmt.Sprintf("\t%v,\n\t", snippet)
			}
			return fmt.Sprintf(", %v,", snippet)
		},
	)
}
