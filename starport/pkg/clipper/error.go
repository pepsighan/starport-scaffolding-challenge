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
	if len(v.missingSelections) > 0 {
		return fmt.Sprintf(
			"%v\n\ncode in improper structure:\n%v",
			v.tracerError.Error(),
			strings.Join(v.missingSelections, "\n"),
		)
	}

	return v.tracerError.Error()
}
