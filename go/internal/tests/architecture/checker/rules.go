package checker

import (
	"path/filepath"
	"strings"
)

// ioStandardLibrary are the standard-library packages that do I/O: network, files, processes, databases.
var ioStandardLibrary = map[string]bool{
	"net": true, "net/http": true, "net/smtp": true, "net/rpc": true, "database/sql": true,
	"os": true, "os/exec": true, "io/ioutil": true, "syscall": true,
}

// forbiddenForDomainPrefixes are the third-party packages the domain never imports: telemetry backends, AI
// frameworks and database drivers (REQ-ARC-002).
var forbiddenForDomainPrefixes = []string{
	"go.opentelemetry.io/", "github.com/prometheus/", "github.com/tmc/langchaingo", "github.com/mattn/go-sqlite3",
	"modernc.org/sqlite", "github.com/lib/pq", "github.com/go-sql-driver/",
}

// checkedFolders are the folders the checker reads, each with the name of the layer whose rules apply to it:
// the four layers of internal/ and, for the consumers of the library, examples/.
var checkedFolders = []struct{ layer_name, folder string }{
	{"domain", "internal/domain"},
	{"application", "internal/application"},
	{"infrastructure", "internal/infrastructure"},
	{"composition_root", "internal/composition_root"},
	{"examples", "examples"},
}

// Check applies every rule to the module that lives in root (the folder that holds internal/ and, if it exists,
// examples/). module_path is the import path of the module.
func Check(root string, module_path string) ([]Violation, error) {
	internal_prefix := module_path + "/internal/"
	var violations []Violation
	for _, checked := range checkedFolders {
		files, err := goFilesUnder(filepath.Join(root, filepath.FromSlash(checked.folder)))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			imports, import_error := importsOfFile(file)
			if import_error != nil {
				return nil, import_error
			}
			relative_file := strings.TrimPrefix(filepath.ToSlash(file), filepath.ToSlash(root)+"/")
			for _, import_path := range imports {
				violations = append(violations, checkImport(checked.layer_name, relative_file, import_path, internal_prefix)...)
			}
		}
	}
	return violations, nil
}

// OfRule keeps the violations of one rule.
func OfRule(violations []Violation, rule string) []Violation {
	var kept []Violation
	for _, violation := range violations {
		if violation.Rule == rule {
			kept = append(kept, violation)
		}
	}
	return kept
}

// checkImport applies the rules of one layer to one import of one file.
func checkImport(layer_name string, relative_file string, import_path string, internal_prefix string) []Violation {
	var violations []Violation
	add := func(rule string) { violations = append(violations, Violation{rule, relative_file, import_path}) }
	layer_path := func(name string) string { return internal_prefix + name }

	switch layer_name {
	case "domain":
		if underPath(import_path, layer_path("application")) || underPath(import_path, layer_path("infrastructure")) ||
			underPath(import_path, layer_path("composition_root")) || forbiddenForDomain(import_path) {
			add(RuleDomainIsolated)
		}
	case "application":
		allowed := isStandardLibrary(import_path) || underPath(import_path, layer_path("domain")) ||
			underPath(import_path, layer_path("application"))
		if !allowed {
			add(RuleApplicationImportsOnlyDomain)
		}
	case "examples":
		if underPath(import_path, strings.TrimSuffix(internal_prefix, "/")) {
			add(RuleConsumerNeverImportsInternal)
		}
	case "infrastructure":
		if underPath(import_path, layer_path("composition_root")) {
			add(RuleInfrastructureNotCompositionRoot)
		}
		if isInsideAdapterButNotExternal(relative_file) && ioStandardLibrary[import_path] {
			add(RuleIOOnlyInExternal)
		}
	}
	inside_adapters := layer_name == "infrastructure" && strings.HasPrefix(relative_file, "internal/infrastructure/outbound/")
	if layer_name != "composition_root" && !inside_adapters && underPath(import_path, layer_path("infrastructure/outbound")) {
		add(RuleAdaptersWiredByCompositionRoot)
	}
	return violations
}

func forbiddenForDomain(import_path string) bool {
	if ioStandardLibrary[import_path] {
		return true
	}
	for _, prefix := range forbiddenForDomainPrefixes {
		if strings.HasPrefix(import_path, prefix) {
			return true
		}
	}
	return false
}

// isInsideAdapterButNotExternal tells whether a file belongs to a concrete outbound adapter but not to its external/ folder.
func isInsideAdapterButNotExternal(relative_file string) bool {
	if !strings.HasPrefix(relative_file, "internal/infrastructure/outbound/") {
		return false
	}
	return !strings.Contains(relative_file, "/external/")
}
