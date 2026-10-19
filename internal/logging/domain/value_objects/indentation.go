package value_objects

import (
	"fmt"
)

// =============================================================================
// IndentationDepth
// =============================================================================

// IndentationDepth places a log entry in the call tree of a request: Indentation grows when the call moves
// to a deeper layer and Depth counts the steps inside one layer.
type IndentationDepth struct {
	Indentation int
	Depth       int
}

// -----------------------------------------------------------------------------
// CONSTRUCTORS
// -----------------------------------------------------------------------------

// NewIndentation creates an IndentationDepth.
func NewIndentation(indentation int, depth int) IndentationDepth {
	return IndentationDepth{
		Indentation: indentation,
		Depth:       depth,
	}
}

// -----------------------------------------------------------------------------
// METHODS
// -----------------------------------------------------------------------------

// NextIndentation returns the position one layer deeper: the indentation grows by one and the depth restarts at 1.
func (indentation_depth IndentationDepth) NextIndentation() IndentationDepth {
	return IndentationDepth{
		Indentation: indentation_depth.Indentation + 1,
		Depth:       1,
	}
}

// NextDepth returns the next step inside the same layer: the depth grows by one.
func (indentation_depth IndentationDepth) NextDepth() IndentationDepth {
	return IndentationDepth{
		Indentation: indentation_depth.Indentation,
		Depth:       indentation_depth.Depth + 1,
	}
}

// String returns the position as "Indentation: <n>, Depth: <n>".
func (indentation_depth IndentationDepth) String() string {
	return fmt.Sprintf("Indentation: %d, Depth: %d", indentation_depth.Indentation, indentation_depth.Depth)
}
