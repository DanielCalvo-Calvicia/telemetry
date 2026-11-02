package helpers

import "strconv"

// =============================================================================
// PLAN ROTATION
// =============================================================================

const (
	// StepRemove is the operation of a step that removes a file (From).
	StepRemove = "remove"
	// StepRename is the operation of a step that renames From to To.
	StepRename = "rename"
)

// Step is one operation of a rotation: Op is StepRemove or StepRename, From is the file it works on and To is the new
// name of a rename (empty for a remove).
type Step struct {
	Op   string
	From string
	To   string
}

// RotatedName returns the name of the rotated file with the index: base.index. Index 1 is the newest.
func RotatedName(base string, index int) string {
	return base + "." + strconv.Itoa(index)
}

// PlanRotation returns the steps, in order, that make room for a new current file. With a max_files of 1 the current file
// is removed. Otherwise the oldest rotated file (index max_files - 1) is removed, each other rotated file moves one index
// up, from the highest, and the current file becomes base.1. Only the names base and base.1 ... base.(max_files - 1) of
// the existing names count; any other name is ignored, and a step is planned only for a file that exists.
func PlanRotation(base string, max_files int, existing []string) []Step {
	exists := make(map[string]bool, len(existing))
	for _, name := range existing {
		exists[name] = true
	}
	steps := []Step{}
	if max_files <= 1 {
		if exists[base] {
			steps = append(steps, Step{Op: StepRemove, From: base})
		}
		return steps
	}
	if oldest := RotatedName(base, max_files-1); exists[oldest] {
		steps = append(steps, Step{Op: StepRemove, From: oldest})
	}
	for index := max_files - 2; index >= 1; index-- {
		if from := RotatedName(base, index); exists[from] {
			steps = append(steps, Step{Op: StepRename, From: from, To: RotatedName(base, index+1)})
		}
	}
	if exists[base] {
		steps = append(steps, Step{Op: StepRename, From: base, To: RotatedName(base, 1)})
	}
	return steps
}
