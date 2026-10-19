package value_objects

// =============================================================================
// Location
// =============================================================================

// Location identifies where a log entry originated.
type Location struct {
	Application string `json:"application"`
	Category    string `json:"category"`
	Function    string `json:"function,omitempty"`
}

// -----------------------------------------------------------------------------
// CONSTRUCTORS
// -----------------------------------------------------------------------------

// NewLocation creates a Location.
func NewLocation(application string, category string, function string) Location {
	return Location{
		Application: application,
		Category:    category,
		Function:    function,
	}
}

// -----------------------------------------------------------------------------
// METHODS
// -----------------------------------------------------------------------------

// WithFunction returns a copy of the Location with the function name set.
func (location Location) WithFunction(function string) Location {
	location.Function = function
	return location
}

// WithCategory returns a copy of the Location with the category set.
func (location Location) WithCategory(category string) Location {
	location.Category = category
	return location
}
