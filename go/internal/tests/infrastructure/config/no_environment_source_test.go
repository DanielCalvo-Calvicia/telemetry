package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// go_module_folder is the folder of the Go module (the folder that holds internal/ and pkg/).
const go_module_folder = "../../../.."

// The library reads no environment and no .env file (ADR-004, REQ-ARC-007): these words must not appear in the sources.
var environment_tokens = []string{"os." + "Getenv", "os." + "LookupEnv", "os." + "Environ", "env" + "config", "go" + "dotenv"}

func Test_no_source_of_the_library_reads_the_environment(t *testing.T) {
	checked_files := 0
	for _, folder_name := range []string{"internal", "pkg"} {
		root := filepath.Join(go_module_folder, folder_name)
		if _, stat_error := os.Stat(root); os.IsNotExist(stat_error) {
			continue
		}
		walk_error := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walk_error error) error {
			if walk_error != nil {
				return walk_error
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, read_error := os.ReadFile(path)
			if read_error != nil {
				return read_error
			}
			checked_files++
			for _, token := range environment_tokens {
				if strings.Contains(string(content), token) {
					t.Errorf("%s contains %q", filepath.ToSlash(path), token)
				}
			}
			return nil
		})
		if walk_error != nil {
			t.Fatal(walk_error)
		}
	}
	if checked_files == 0 {
		t.Fatal("no source file was checked")
	}
}
