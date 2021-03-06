package clipper

import (
	"bufio"
	"strings"

	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

// lineOffsetMap is the offset from the base of the code for each line.
type lineOffsetMap map[int]int

// lineOffsetMapOfFile gets the offset of each of the lines from the base of the code.
func lineOffsetMapOfFile(code string) (lineOffsetMap, error) {
	offsetMap := lineOffsetMap{}
	line := 0

	scanner := bufio.NewScanner(strings.NewReader(code))
	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		line += 1
		if line == 1 {
			// The offset is 0-indexed and always points to the character next to the text.
			offsetMap[line] = len(scanner.Text())
		} else {
			// The offset count will account for the new line char preceding it.
			offsetMap[line] = offsetMap[line-1] + len(scanner.Text()) + 1
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return offsetMap, nil
}

// offsetForProtoSourcePos converts the Proto's source position to an offset position.
// The ast.SourcePos also has an Offset field, but it is incorrectly implemented (or that
// I don't understand what the offset actually means in its context).
func offsetForProtoSourcePos(offsetMap lineOffsetMap, pos *ast.SourcePos) OffsetPosition {
	if pos == nil {
		return NoOffsetPosition
	}

	return OffsetPosition(offsetMap[pos.Line-1] + pos.Col)
}
