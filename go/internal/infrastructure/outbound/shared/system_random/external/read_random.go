// Package external holds the call of the system random source to the machine; it is the only one that reads it.
package external

import "math/rand/v2"

// ReadFloat64 returns a number in [0, 1) from the random source of the runtime, which is safe for concurrent use and
// seeded by the system.
func ReadFloat64() float64 {
	return rand.Float64()
}
