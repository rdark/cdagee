package target

import "fmt"

// ParseError indicates a malformed or unreadable cdagee.json file.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse %s: %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// DanglingRefError indicates a depends_on reference to a target that does not exist.
type DanglingRefError struct {
	Target    string
	Reference string
}

func (e *DanglingRefError) Error() string {
	return fmt.Sprintf("target %q depends on %q, which does not exist", e.Target, e.Reference)
}

// CycleError indicates a cycle was detected in the target dependency graph.
type CycleError struct {
	Err error
}

func (e *CycleError) Error() string {
	return e.Err.Error()
}

func (e *CycleError) Unwrap() error {
	return e.Err
}

// DuplicateIDError indicates that multiple targets share the same ID.
type DuplicateIDError struct {
	ID string
}

func (e *DuplicateIDError) Error() string {
	return fmt.Sprintf("duplicate target ID %q", e.ID)
}
