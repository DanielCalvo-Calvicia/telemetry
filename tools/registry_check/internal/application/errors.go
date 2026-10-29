package application

import "errors"

// The errors of the use case and of the ports. Each one is wrapped with the path of the file it is about.
var (
	// ErrReadRegistry is returned when the registry file cannot be read.
	ErrReadRegistry = errors.New("cannot read the registry")
	// ErrInvalidYAML is returned for a registry that is not valid YAML of the expected shape.
	ErrInvalidYAML = errors.New("invalid YAML")
	// ErrReadSpec is returned when the specification file cannot be read.
	ErrReadSpec = errors.New("cannot read the specification")
	// ErrNoRequirementsInSpec is returned for a specification that defines no requirement, which would make the check
	// pass without checking anything.
	ErrNoRequirementsInSpec = errors.New("the specification defines no requirement identifier")
	// ErrReadTasks is returned when the tasks folder cannot be read.
	ErrReadTasks = errors.New("cannot read the tasks folder")
	// ErrCheckTestFile is returned when a listed test file cannot be inspected for a reason other than not existing.
	ErrCheckTestFile = errors.New("cannot inspect the test file")
)
