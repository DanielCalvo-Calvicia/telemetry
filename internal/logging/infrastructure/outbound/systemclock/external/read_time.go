package external

import "time"

// ReadTime returns the current time of the machine. It is the only file in the codebase that calls time.Now.
func ReadTime() time.Time {
	return time.Now()
}
