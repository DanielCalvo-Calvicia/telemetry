package composition_root_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// go_module_root is the folder of go.mod, seen from this package.
const go_module_root = "../../.."

// allowed_global_variables are the package-level variables that are not an assertion, a sentinel or a compiled
// pattern, each with the reason it is not a mutable global. The list is explicit and small on purpose: a new entry
// needs a reason.
var allowed_global_variables = map[string]string{
	"SensitiveFieldNames":   "a read-only table of field names; nothing writes it, the masker copies it",
	"sentinel_codes":        "a read-only table that maps each sentinel to its code",
	"event_id_group_widths": "a read-only array of the widths of the groups of an event id",
	"field_name_separators": "a strings.Replacer, immutable once built and safe for concurrent use",
	"default_field_names":   "the normalized copy of SensitiveFieldNames, built once at start and never written",
	"event_validators":      "the generated table of validators by event type (generated file)",
	"time_type":             "the reflect.Type of time.Time, a descriptor that is never written (canonical_json)",
	"json_number_type":      "the reflect.Type of json.Number, a descriptor that is never written (canonical_json)",
	"json_marshaler_type":   "the reflect.Type of json.Marshaler, a descriptor that is never written (canonical_json)",
	"text_marshaler_type":   "the reflect.Type of encoding.TextMarshaler, a descriptor that is never written (canonical_json)",
}

// non_test_go_files lists the .go files of the core that are not tests: internal/ (without internal/tests) and pkg/.
func non_test_go_files(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range []string{filepath.Join(go_module_root, "internal"), filepath.Join(go_module_root, "pkg")} {
		walk_error := filepath.Walk(root, func(path string, info os.FileInfo, walk_error error) error {
			if walk_error != nil {
				return walk_error
			}
			slash_path := filepath.ToSlash(path)
			if info.IsDir() && strings.HasSuffix(slash_path, "internal/tests") {
				return filepath.SkipDir
			}
			if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if walk_error != nil {
			t.Fatal(walk_error)
		}
	}
	if len(files) == 0 {
		t.Fatal("no source file found")
	}
	return files
}

func parse_source(t *testing.T, path string) *ast.File {
	t.Helper()
	file, parse_error := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if parse_error != nil {
		t.Fatalf("%s: %v", path, parse_error)
	}
	return file
}

func is_generated(file *ast.File) bool {
	for _, comment_group := range file.Comments {
		for _, comment := range comment_group.List {
			if strings.HasPrefix(comment.Text, "// Code generated") && strings.HasSuffix(comment.Text, "DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}

func Test_no_source_file_registers_itself_with_an_init_function(t *testing.T) {
	for _, path := range non_test_go_files(t) {
		for _, declaration := range parse_source(t, path).Decls {
			if function, is_function := declaration.(*ast.FuncDecl); is_function && function.Recv == nil && function.Name.Name == "init" {
				t.Errorf("%s declares func init(): an adapter never registers itself (REQ-ARC-013)", path)
			}
		}
	}
}

// is_call_of reports whether the expression is a call of package.function.
func is_call_of(expression ast.Expr, package_name string, function_name string) bool {
	call, is_call := expression.(*ast.CallExpr)
	if !is_call {
		return false
	}
	selector, is_selector := call.Fun.(*ast.SelectorExpr)
	if !is_selector {
		return false
	}
	identifier, is_identifier := selector.X.(*ast.Ident)
	return is_identifier && identifier.Name == package_name && selector.Sel.Name == function_name
}

// is_allowed_by_shape reports whether the value of a package-level variable is a sentinel made with errors.New, an
// alias of the Err sentinel of another package or a compiled regular expression.
func is_allowed_by_shape(value ast.Expr) bool {
	if is_call_of(value, "errors", "New") || is_call_of(value, "regexp", "MustCompile") {
		return true
	}
	selector, is_selector := value.(*ast.SelectorExpr)
	return is_selector && strings.HasPrefix(selector.Sel.Name, "Err")
}

func Test_no_package_level_variable_is_a_mutable_global(t *testing.T) {
	used_exceptions := map[string]bool{}
	for _, path := range non_test_go_files(t) {
		file := parse_source(t, path)
		if is_generated(file) {
			continue
		}
		for _, declaration := range file.Decls {
			general, is_general := declaration.(*ast.GenDecl)
			if !is_general || general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				value_spec := spec.(*ast.ValueSpec)
				for index, name := range value_spec.Names {
					if name.Name == "_" {
						continue // var _ Port = (*Adapter)(nil): a compile-time assertion
					}
					if index < len(value_spec.Values) && is_allowed_by_shape(value_spec.Values[index]) {
						continue
					}
					if _, listed := allowed_global_variables[name.Name]; listed {
						used_exceptions[name.Name] = true
						continue
					}
					t.Errorf("%s: the package-level variable %s is not an assertion, a sentinel or a compiled pattern and is not in the allow-list", path, name.Name)
				}
			}
		}
	}
	for name := range used_exceptions {
		if allowed_global_variables[name] == "" {
			t.Errorf("the exception %s has no reason", name)
		}
	}
}

func Test_the_go_mod_of_the_core_has_no_require_line(t *testing.T) {
	content, read_error := os.ReadFile(filepath.Join(go_module_root, "go.mod"))
	if read_error != nil {
		t.Fatal(read_error)
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "require") {
			t.Fatalf("go.mod has a require line (%q): the core has no third-party dependency (REQ-ARC-005)", strings.TrimSpace(line))
		}
	}
}
