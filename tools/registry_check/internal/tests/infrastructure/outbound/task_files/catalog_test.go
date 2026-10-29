package task_files_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/task_files"
)

type fake_folder struct {
	files      []string
	list_error error
	calls      *int
}

func (folder fake_folder) Files() ([]string, error) {
	*folder.calls++
	return folder.files, folder.list_error
}
func (folder fake_folder) Path() string { return "docs/tasks" }

func TestATaskExistsWhenItsFileNameStartsWithItsIdentifier(test *testing.T) {
	calls := 0
	catalog := task_files.NewCatalog(fake_folder{calls: &calls, files: []string{
		"README.md", "notes_T-09.md", "T-04_logging/T-04_logging.md", "T-04_logging/T-04.7_replace-the-old-library.md", "T-04_logging/T-04.7_replace.txt", "T-12_x/T-12.10_ten.md",
	}})
	for task_id, want := range map[string]bool{"T-04": true, "T-04.7": true, "T-12.10": true, "T-12.1": false, "T-09": false, "T-04.8": false} {
		if exists, find_error := catalog.Exists(task_id); find_error != nil || exists != want {
			test.Errorf("%s: %v %v", task_id, exists, find_error)
		}
	}
	if calls != 1 {
		test.Errorf("the folder is listed once, not %d times", calls)
	}
}

func TestAFolderThatCannotBeListedIsAnErrorThatNamesIt(test *testing.T) {
	calls := 0
	denied := errors.New("denied")
	_, find_error := task_files.NewCatalog(fake_folder{calls: &calls, list_error: denied}).Exists("T-01")
	if !errors.Is(find_error, application.ErrReadTasks) || !errors.Is(find_error, denied) || !strings.Contains(find_error.Error(), "docs/tasks") {
		test.Errorf("%v", find_error)
	}
}
