package operations

import (
	"regexp"

	domain "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
)

// NewJWTDetector creates the detector of JSON Web Tokens; the whole token is hidden.
func NewJWTDetector() TextDetector {
	return NewTextDetector(domain.DetectorJWT, regexp.MustCompile(domain.JWTPattern), domain.MaskedText)
}
