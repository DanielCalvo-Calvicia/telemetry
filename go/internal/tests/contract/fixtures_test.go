package contract_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
)

// shared_fixture_folders are the folders of integration-tests/fixtures that this test reads. Adding a file to one of
// them needs no change here: every file is a case and its "kind" says how it is checked (REQ-CNF-005).
var shared_fixture_folders = []string{"envelope", "events"}

const shared_fixtures_root = "../../../../integration-tests/fixtures"

// shared_fixture is one case file. See integration-tests/fixtures/README.md.
type shared_fixture struct {
	Description     string   `json:"description"`
	Kind            string   `json:"kind"`
	Input           any      `json:"input"`
	Expected        *string  `json:"expected"`
	ErrorCode       *string  `json:"error_code"`
	MessageContains []string `json:"message_contains"`
}

func load_shared_fixtures(test *testing.T) map[string]shared_fixture {
	test.Helper()
	cases := map[string]shared_fixture{}
	for _, folder := range shared_fixture_folders {
		pattern := filepath.Join(shared_fixtures_root, folder, "*.json")
		paths, err := filepath.Glob(pattern)
		if err != nil || len(paths) == 0 {
			test.Fatalf("no fixture files found by %s: %v", pattern, err)
		}
		for _, path := range paths {
			content, read_error := os.ReadFile(path)
			if read_error != nil {
				test.Fatalf("cannot read %s: %v", path, read_error)
			}
			var fixture shared_fixture
			if decode_error := json.Unmarshal(content, &fixture); decode_error != nil {
				test.Fatalf("%s is not a valid fixture: %v", path, decode_error)
			}
			cases[folder+"/"+filepath.Base(path)] = fixture
		}
	}
	return cases
}

func code_of(test *testing.T, name string, err error) string {
	test.Helper()
	var classified *domain.ClassifiedError
	if !errors.As(err, &classified) {
		test.Fatalf("%s: the error %v carries no code", name, err)
	}
	return classified.Code
}

func TestTheSharedFixturesGiveTheExpectedResultsInGo(test *testing.T) {
	for name, fixture := range load_shared_fixtures(test) {
		test.Run(name, func(test *testing.T) {
			switch fixture.Kind {
			case "canonical":
				got, err := canonical_json.CanonicalJSON(fixture.Input)
				if err != nil {
					test.Fatalf("serializing failed: %v", err)
				}
				if fixture.Expected == nil {
					test.Fatalf("a canonical fixture needs an expected text")
				}
				// The bytes are compared exactly, not as parsed JSON.
				if string(got) != *fixture.Expected {
					test.Fatalf("bytes differ\n got: %s\nwant: %s", got, *fixture.Expected)
				}
			case "time":
				text, ok := fixture.Input.(string)
				if !ok {
					test.Fatalf("a time fixture needs a text input")
				}
				instant, parse_error := time.Parse(time.RFC3339Nano, text)
				if parse_error != nil {
					test.Fatalf("cannot parse %q: %v", text, parse_error)
				}
				formatted, err := canonical_json.FormatUTCTime(instant)
				if fixture.ErrorCode != nil {
					if err == nil {
						test.Fatalf("%q must be rejected, got %q", text, formatted)
					}
					if got := code_of(test, name, err); got != *fixture.ErrorCode {
						test.Fatalf("error code %q, want %q", got, *fixture.ErrorCode)
					}
					return
				}
				if err != nil || fixture.Expected == nil || formatted != *fixture.Expected {
					test.Fatalf("formatted %q (error %v), want %v", formatted, err, fixture.Expected)
				}
			case "validation":
				input, ok := fixture.Input.(map[string]any)
				if !ok {
					test.Fatalf("a validation fixture needs an object input")
				}
				event_type, _ := input["event_type"].(string)
				err := event_validation.ValidateEvent(event_type, input["data"])
				if fixture.ErrorCode == nil {
					if err != nil {
						test.Fatalf("the event must be valid, got %v", err)
					}
					return
				}
				if err == nil {
					test.Fatalf("the event must be rejected with %q", *fixture.ErrorCode)
				}
				if got := code_of(test, name, err); got != *fixture.ErrorCode {
					test.Fatalf("error code %q, want %q", got, *fixture.ErrorCode)
				}
				for _, part := range fixture.MessageContains {
					if !strings.Contains(err.Error(), part) {
						test.Fatalf("the message %q does not contain %q", err.Error(), part)
					}
				}
			default:
				test.Fatalf("unknown fixture kind %q (a typo must not be skipped silently)", fixture.Kind)
			}
		})
	}
}
