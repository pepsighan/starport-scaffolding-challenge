package clipper

import (
	"fmt"
	"strings"
)

// ValidationError checks if the selectors run well or the placeholders exist.
type ValidationError struct {
	missingSelections []string
	tracerError       error
}

// Error implements the Error interface for ValidationError.
func (v *ValidationError) Error() string {
	err := ""

	if v.tracerError != nil {
		err += v.tracerError.Error() + "\n\n"
	}

	if len(v.missingSelections) > 0 {
		err += fmt.Sprintf(
			"code in improper structure:\n%v",
			strings.Join(v.missingSelections, "\n"),
		)
	}

	return err
}
