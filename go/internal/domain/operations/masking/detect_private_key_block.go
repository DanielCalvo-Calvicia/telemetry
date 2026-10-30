package masking

import (
	"regexp"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// NewPrivateKeyBlockDetector creates the detector of PEM private key blocks; the whole block is hidden, up to
// the end of the text when the block is cut.
func NewPrivateKeyBlockDetector() TextDetector {
	return NewTextDetector(domain.DetectorPrivateKeyBlock, regexp.MustCompile(domain.PrivateKeyBlockPattern), domain.MaskedText)
}
