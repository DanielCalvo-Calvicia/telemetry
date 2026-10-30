package masking

import (
	"regexp"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// NewCredentialPairDetector creates the detector of credentials written as a pair (password=..., "token": "...",
// X-Amz-Signature=...); the name is kept and the value is hidden.
func NewCredentialPairDetector() TextDetector {
	return NewTextDetector(domain.DetectorCredentialPair, regexp.MustCompile(domain.CredentialPairPattern), domain.MaskKeepFirstGroup)
}
