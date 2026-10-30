package masking

import (
	"regexp"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// NewAWSAccessKeyDetector creates the detector of AWS access key IDs (AKIA..., ASIA..., ...); the whole key is hidden.
func NewAWSAccessKeyDetector() TextDetector {
	return NewTextDetector(domain.DetectorAWSAccessKey, regexp.MustCompile(domain.AWSAccessKeyPattern), domain.MaskedText)
}
