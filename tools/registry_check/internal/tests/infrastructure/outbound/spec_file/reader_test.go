package spec_file_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/spec_file"
)

type fake_file struct {
	content    string
	read_error error
}

func (file fake_file) Read() ([]byte, error) { return []byte(file.content), file.read_error }
func (file fake_file) Path() string          { return "docs/proposals/spec.md" }

func TestOnlyABoldDefinitionAtTheStartOfALineIsAnIdentifier(test *testing.T) {
	content := "**REQ-ZZ-002.** two\r\n**REQ-AAAA-001.** one\r\nsee **REQ-XX-003.** inline\n" +
		"REQ-XX-004. not bold\n**REQ-X-005.** one letter area\n**REQ-AAAAA-006.** five letters\n**REQ-AA-07.** two digits\n" +
		"**REQ-AAAA-001.** repeated\n> **REQ-XX-009.** quoted\n**REQ-QQ-010** without the dot\n"
	spec, read_error := spec_file.NewReader(fake_file{content: content}).Read()
	if read_error != nil {
		test.Fatal(read_error)
	}
	if strings.Join(spec.IDs, ",") != "REQ-AAAA-001,REQ-ZZ-002" || spec.Path != "docs/proposals/spec.md" {
		test.Errorf("%+v", spec)
	}
}

func TestAnUnreadableSpecificationNamesTheFile(test *testing.T) {
	_, read_error := spec_file.NewReader(fake_file{read_error: errors.New("gone")}).Read()
	if !errors.Is(read_error, application.ErrReadSpec) || !strings.Contains(read_error.Error(), "docs/proposals/spec.md") {
		test.Errorf("%v", read_error)
	}
}
