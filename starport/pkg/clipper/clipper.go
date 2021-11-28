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
	*placeholder.Tracer
	// Missing clipper selections that is needed for the clipper to work.
	missingSelections       []int
	missingSelectionFiles   []string
	missingSelectionOptions []SelectOptions
}

// New creates a new clipper.
func New() *Clipper {
	return &Clipper{Tracer: placeholder.New()}
}

// Err if any selections or placeholders were missing during execution.
func (c *Clipper) Err() error {
	var missingSelections []string

	if len(c.missingSelections) > 0 {
		for id, sel := range c.missingSelections {
			file := c.missingSelectionFiles[id]
			options := c.missingSelectionOptions[id]

			switch sel {
			case ProtoSelectNewImportPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find position to add new import in %v", file),
				)
			case ProtoSelectNewMessageFieldPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find message %v in %v", options["name"], file),
				)
			case ProtoSelectNewServiceMethodPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find service %v in %v", options["name"], file),
				)
			case ProtoSelectNewOneOfFieldPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find message %v with oneof field %v in %v",
						options["messageName"], options["oneOfName"], file),
				)
			case ProtoSelectLastPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find last position of file in %v", file),
				)
			case GoSelectNewImportPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find position to add new import in %v", file),
				)
			case GoSelectNewGlobalPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find position for global declaration in %v", file),
				)
			case GoSelectBeforeFunctionReturnsPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find function %v in %v", options["functionName"], file),
				)
			case GoSelectStartOfFunctionPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find function %v in %v", options["functionName"], file),
				)
			case GoSelectReturningFunctionCallNewArgumentPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find function %v which is returning value with a function call in %v",
						options["functionName"], file),
				)
			case GoSelectReturningCompositeNewArgumentPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find function %v which is returning value with a map/struct call in %v",
						options["functionName"], file),
				)
			case GoSelectStructNewFieldPosition.id:
				missingSelections = append(
					missingSelections,
					fmt.Sprintf("◦ cannot find struct %v in %v", options["structName"], file),
				)
			}
		}
	}

	tracerError := c.Tracer.Err()
	if tracerError != nil || len(missingSelections) != 0 {
		return &ValidationError{
			tracerError:       tracerError,
			missingSelections: missingSelections,
		}
	}

	return nil
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
		// Do nothing and return the code as is. The errors are accumulated by the clipper.
		c.missingSelections = append(c.missingSelections, selector.id)
		c.missingSelectionFiles = append(c.missingSelectionFiles, path)
		c.missingSelectionOptions = append(c.missingSelectionOptions, options)
		return code, nil
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
			d := data.(GoReturningFunctionCallNewArgumentPositionData)
			if !d.HasArguments {
				return fmt.Sprintf("%v,", snippet)
			}
			if d.HasTrailingComma {
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
			d := data.(GoReturningCompositeNewArgumentPositionData)
			if !d.HasArguments {
				return fmt.Sprintf("%v,", snippet)
			}
			if d.HasTrailingComma {
				return fmt.Sprintf("\t%v,\n\t", snippet)
			}
			return fmt.Sprintf(", %v,", snippet)
		},
	)
}
