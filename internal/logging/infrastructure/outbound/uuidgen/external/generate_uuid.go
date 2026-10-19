package external

import "github.com/google/uuid"

// GenerateUUID returns a new random (version 4) UUID as text. It fails only when the system random source
// fails. This is the only file in the codebase that imports google/uuid.
func GenerateUUID() (string, error) {
	new_uuid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return new_uuid.String(), nil
}
