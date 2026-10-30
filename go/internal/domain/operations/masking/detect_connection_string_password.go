package masking

import (
	"regexp"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// NewConnectionStringPasswordDetector creates the detector of the password inside a URL with credentials
// (scheme://user:password@host); the rest of the URL is kept and only the password is hidden.
func NewConnectionStringPasswordDetector() TextDetector {
	return NewTextDetector(domain.DetectorConnectionStringPassword, regexp.MustCompile(domain.ConnectionStringPasswordPattern), domain.MaskKeepOuterGroups)
}
