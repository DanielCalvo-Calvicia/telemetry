package diagnostics_reason_test

import (
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/diagnostics_reason"
)

func Test_the_reason_keeps_only_the_allowed_characters_and_replaces_the_rest(t *testing.T) {
	cases := map[string]string{
		"":                              "unspecified",
		"status_503":                    "status_503",
		"a.b:c-d_E9":                    "a.b:c-d_E9",
		"has space":                     "has_space",
		"http://host/path?token=s3cr3t": "http:__host_path_token_s3cr3t",
		"café":                          "caf_",
		"line\nbreak":                   "line_break",
	}
	for reason, expected := range cases {
		if got := diagnostics_reason.SanitizeReason(reason); got != expected {
			t.Errorf("%q: got %q, expected %q", reason, got, expected)
		}
	}
}

func Test_the_reason_is_cut_to_sixty_four_characters(t *testing.T) {
	cleaned := diagnostics_reason.SanitizeReason(strings.Repeat("a", 200))
	if cleaned != strings.Repeat("a", diagnostics_reason.MaxReasonChars) || len(cleaned) != 64 {
		t.Fatalf("%d characters", len(cleaned))
	}
	if cut := diagnostics_reason.SanitizeReason(strings.Repeat("é", 100)); cut != strings.Repeat("_", 64) {
		t.Fatalf("%q", cut)
	}
}
