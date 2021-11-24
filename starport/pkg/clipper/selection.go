package clipper

// SelectOptions are the options needed to configure a particular selector.
type SelectOptions map[string]string

// OffsetPosition is the position in code from its base.
type OffsetPosition int

// NoOffsetPosition is when there is no possible position.
const NoOffsetPosition OffsetPosition = -1

// PositionSelectorResult contains position in code after a successful selection.
type PositionSelectorResult struct {
	OffsetPosition OffsetPosition
	// Any additional piece of data collected during a selection.
	Data interface{}
}

// positionSelectorID is a counter for the id to make comparison between selectors easy.
var positionSelectorID = 0

// PositionSelector is a configurable selector which can select a position in code.
type PositionSelector struct {
	id   int
	call func(path, code string, options SelectOptions) (*PositionSelectorResult, error)
}
