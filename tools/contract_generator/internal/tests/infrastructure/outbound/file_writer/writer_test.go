package file_writer_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/file_writer"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/file_writer/external"
)

type failing_disk struct{}

func (failing_disk) WriteFile(string, string, []byte) error { return errors.New("disk full") }

func TestTheFilesAreWrittenWithTheExactBytesAndTheFolderIsCreated(test *testing.T) {
	directory := filepath.Join(test.TempDir(), "a", "b")
	files := []domain.GeneratedFile{
		{Name: "one.txt", Content: []byte("line\n")},
		{Name: "two.txt", Content: []byte("other\n")},
	}
	if write_error := file_writer.NewWriter(external.Disk{}, directory).Write(files); write_error != nil {
		test.Fatal(write_error)
	}
	for _, generated := range files {
		content, read_error := os.ReadFile(filepath.Join(directory, generated.Name))
		if read_error != nil || string(content) != string(generated.Content) {
			test.Errorf("%s: %q, %v", generated.Name, content, read_error)
		}
	}
}

func TestAnExistingFileIsReplaced(test *testing.T) {
	directory := test.TempDir()
	writer := file_writer.NewWriter(external.Disk{}, directory)
	_ = writer.Write([]domain.GeneratedFile{{Name: "one.txt", Content: []byte("old content\n")}})
	_ = writer.Write([]domain.GeneratedFile{{Name: "one.txt", Content: []byte("new\n")}})
	if content, _ := os.ReadFile(filepath.Join(directory, "one.txt")); string(content) != "new\n" {
		test.Errorf("content %q", content)
	}
}

func TestAWriteFailureNamesTheFile(test *testing.T) {
	write_error := file_writer.NewWriter(failing_disk{}, "out").Write([]domain.GeneratedFile{{Name: "one.txt"}})
	if !errors.Is(write_error, application.ErrWriteOutput) || !strings.Contains(write_error.Error(), "out/one.txt") {
		test.Errorf("error %v", write_error)
	}
}

func TestAFolderThatCannotBeCreatedIsAnError(test *testing.T) {
	blocker := filepath.Join(test.TempDir(), "blocker")
	if write_error := os.WriteFile(blocker, []byte("a file, not a folder"), 0o644); write_error != nil {
		test.Fatal(write_error)
	}
	write_error := file_writer.NewWriter(external.Disk{}, filepath.Join(blocker, "inside")).Write([]domain.GeneratedFile{{Name: "one.txt"}})
	if !errors.Is(write_error, application.ErrWriteOutput) {
		test.Errorf("error %v", write_error)
	}
}
