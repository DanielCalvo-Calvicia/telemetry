package masking

import (
	"regexp"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// NewBearerCredentialDetector creates the detector of the credential that follows an authorization scheme
// (Bearer, Basic, Token); the scheme is kept and the credential is hidden.
func NewBearerCredentialDetector() TextDetector {
	return NewTextDetector(domain.DetectorBearerCredential, regexp.MustCompile(domain.BearerCredentialPattern), domain.MaskKeepFirstGroup)
}
