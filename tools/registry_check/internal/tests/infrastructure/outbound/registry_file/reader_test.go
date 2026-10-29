package registry_file_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/registry_file"
)

type fake_file struct {
	content    string
	read_error error
}

func (file fake_file) Read() ([]byte, error) { return []byte(file.content), file.read_error }
func (file fake_file) Path() string          { return "contracts/capabilities/registry.yaml" }

const document = `registry_version: 1
capabilities:
  - id: alpha
    title: Alpha
    level: beta
    status: in_progress
    description: The first.
    public_api: [one]
    schemas: [two]
    configuration: [three]
    errors: [four]
    ports: [five]
    adapters: [six]
    languages:
      go: { state: passing }
      python: { state: not_applicable, reason: Go only }
    requirements:
      - id: REQ-AAA-001
        tests:
          - { language: go, path: tests/a_test.go }
      - id: REQ-AAA-002
        pending: T-01.1
`

func TestEveryFieldReachesTheModel(test *testing.T) {
	registry, read_error := registry_file.NewReader(fake_file{content: document}).Read()
	if read_error != nil {
		test.Fatal(read_error)
	}
	if registry.Path != "contracts/capabilities/registry.yaml" || registry.Version != 1 || len(registry.Capabilities) != 1 {
		test.Fatalf("%+v", registry)
	}
	capability := registry.Capabilities[0]
	if capability.ID != "alpha" || capability.Title != "Alpha" || capability.Level != "beta" || capability.Status != "in_progress" || capability.Description != "The first." {
		test.Errorf("%+v", capability)
	}
	var lists []string
	for _, list := range [][]string{capability.PublicAPI, capability.Schemas, capability.Configuration, capability.Errors, capability.Ports, capability.Adapters} {
		lists = append(lists, list...)
	}
	if strings.Join(lists, ",") != "one,two,three,four,five,six" {
		test.Errorf("lists %v", lists)
	}
	if capability.Go != (domain.LanguageState{State: "passing"}) || capability.Python != (domain.LanguageState{State: "not_applicable", Reason: "Go only"}) {
		test.Errorf("languages %+v %+v", capability.Go, capability.Python)
	}
	if len(capability.Requirements) != 2 || capability.Requirements[0].Tests[0] != (domain.TestRef{Language: "go", Path: "tests/a_test.go"}) || capability.Requirements[1].Pending != "T-01.1" {
		test.Errorf("%+v", capability.Requirements)
	}
}

func TestAnUnreadableFileAndBadYAMLNameTheFile(test *testing.T) {
	disk_error := errors.New("access denied")
	_, read_error := registry_file.NewReader(fake_file{read_error: disk_error}).Read()
	if !errors.Is(read_error, application.ErrReadRegistry) || !errors.Is(read_error, disk_error) || !strings.Contains(read_error.Error(), "contracts/capabilities/registry.yaml") {
		test.Errorf("%v", read_error)
	}
	for _, content := range []string{"capabilities: [", "capabilities: 3", "registry_version: one", "misspelled: 1", ""} {
		_, read_error = registry_file.NewReader(fake_file{content: content}).Read()
		if !errors.Is(read_error, application.ErrInvalidYAML) || !strings.Contains(read_error.Error(), "contracts/capabilities/registry.yaml") {
			test.Errorf("%q: %v", content, read_error)
		}
	}
}
