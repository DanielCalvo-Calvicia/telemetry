package test_files_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/infrastructure/outbound/test_files"
)

type fake_disk struct {
	files      map[string]bool
	stat_error error
}

func (disk fake_disk) IsFile(relative_path string) (bool, error) {
	return disk.files[relative_path], disk.stat_error
}

func TestTheFinderAnswersWhatTheDiskSays(test *testing.T) {
	finder := test_files.NewFinder(fake_disk{files: map[string]bool{"tests/a_test.go": true}})
	if exists, find_error := finder.Exists("tests/a_test.go"); !exists || find_error != nil {
		test.Errorf("%v %v", exists, find_error)
	}
	if exists, find_error := finder.Exists("tests/b_test.go"); exists || find_error != nil {
		test.Errorf("%v %v", exists, find_error)
	}
	denied := errors.New("denied")
	if _, find_error := test_files.NewFinder(fake_disk{stat_error: denied}).Exists("x"); !errors.Is(find_error, denied) {
		test.Errorf("%v", find_error)
	}
}
