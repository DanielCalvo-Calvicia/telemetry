package masking

import (
	"regexp"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// NewJWTDetector creates the detector of JSON Web Tokens; the whole token is hidden.
func NewJWTDetector() TextDetector {
	return NewTextDetector(domain.DetectorJWT, regexp.MustCompile(domain.JWTPattern), domain.MaskedText)
}
