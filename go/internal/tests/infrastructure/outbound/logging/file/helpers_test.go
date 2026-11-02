package file_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	console_helpers "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/console/helpers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file/external"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/logging/file/helpers"
)

func Test_encode_json_line_is_the_canonical_json_and_a_line_feed_like_the_console(t *testing.T) {
	event := log_event(value_objects.SeverityInfo, "m")
	line, encode_error := helpers.EncodeJSONLine(event)
	if encode_error != nil {
		t.Fatal(encode_error)
	}
	expected := `{"data":{"category":"c","message":"m"},"event_id":"01900000-0000-7000-8000-000000000001","event_type":"log.entry.recorded",` +
		`"occurred_at":"2026-10-28T10:00:00.123456Z","recorded_at":"2026-10-28T10:00:00.123456Z","resource":{"instance_id":"","service_name":""},` +
		`"schema_version":"0.1","severity":"INFO"}` + "\n"
	if string(line) != expected {
		t.Fatalf("got\n%s\nexpected\n%s", line, expected)
	}
	console_line, _ := console_helpers.EncodeJSONLine(event)
	if string(console_line) != string(line) {
		t.Fatal("the file and the console must write the same bytes")
	}
	bad_event := log_event(value_objects.SeverityInfo, "m")
	bad_event.Data = map[string]any{"value": math.NaN()}
	if _, encode_error := helpers.EncodeJSONLine(bad_event); encode_error == nil {
		t.Error("a NaN in the data must be an error")
	}
}

func Test_plan_rotation_with_one_file_removes_the_current_file_only_when_it_exists(t *testing.T) {
	if got := helpers.PlanRotation("a.jsonl", 1, []string{"a.jsonl", "a.jsonl.1"}); !reflect.DeepEqual(got, []helpers.Step{{Op: "remove", From: "a.jsonl"}}) {
		t.Errorf("got %v", got)
	}
	if got := helpers.PlanRotation("a.jsonl", 1, []string{"other"}); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func Test_plan_rotation_ignores_other_names_and_indexes_out_of_range(t *testing.T) {
	got := helpers.PlanRotation("a.jsonl", 2, []string{"a.jsonl", "a.jsonl.1", "a.jsonl.2", "a.jsonl.x", "b.jsonl.1", "a.jsonl.bak"})
	expected := []helpers.Step{
		{Op: "remove", From: "a.jsonl.1"},
		{Op: "rename", From: "a.jsonl", To: "a.jsonl.1"},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("got %v, expected %v", got, expected)
	}
	if got := helpers.PlanRotation("a.jsonl", 3, nil); len(got) != 0 {
		t.Errorf("nothing exists, got %v", got)
	}
}

func Test_plan_quota_never_plans_the_current_file_and_handles_one_file(t *testing.T) {
	if got := helpers.PlanQuota("a.jsonl", 1, map[string]int64{"a.jsonl": 5000}, 10); len(got) != 0 {
		t.Errorf("the current file must never be planned, got %v", got)
	}
	got := helpers.PlanQuota("a.jsonl", 3, map[string]int64{"a.jsonl": 500, "a.jsonl.2": 500, "a.jsonl.9": 9000, "other": 9000}, 600)
	if !reflect.DeepEqual(got, []string{"a.jsonl.2"}) {
		t.Errorf("got %v", got)
	}
}

func Test_the_external_calls_on_a_temporary_directory(t *testing.T) {
	directory := t.TempDir()
	path := directory + "/sub/file.txt"
	if directory_error := external.EnsureDirectory(directory + "/sub"); directory_error != nil {
		t.Fatal(directory_error)
	}
	if _, exists, size_error := external.FileSize(path); exists || size_error != nil {
		t.Fatalf("a missing file: exists=%v error=%v", exists, size_error)
	}
	if touch_error := external.TouchAppend(path); touch_error != nil {
		t.Fatal(touch_error)
	}
	if size, exists, _ := external.FileSize(path); !exists || size != 0 {
		t.Fatalf("a touched file is empty, got %d %v", size, exists)
	}
	if append_error := external.AppendLine(path, []byte("abc\n")); append_error != nil {
		t.Fatal(append_error)
	}
	if append_error := external.AppendLine(path, []byte("de\n")); append_error != nil {
		t.Fatal(append_error)
	}
	if size, _, _ := external.FileSize(path); size != 7 {
		t.Fatalf("the size is %d", size)
	}
	names, list_error := external.ListNames(directory + "/sub")
	if list_error != nil || !reflect.DeepEqual(names, []string{"file.txt"}) {
		t.Fatalf("names %v, error %v", names, list_error)
	}
	if rename_error := external.Rename(path, path+".1"); rename_error != nil {
		t.Fatal(rename_error)
	}
	if remove_error := external.Remove(path + ".1"); remove_error != nil {
		t.Fatal(remove_error)
	}
	if _, list_error := external.ListNames(directory + "/missing"); list_error == nil {
		t.Error("a missing directory cannot be listed")
	}
	if append_error := external.AppendLine(directory+"/missing/x", []byte("x")); append_error == nil {
		t.Error("a missing directory cannot be appended to")
	}
	if touch_error := external.TouchAppend(directory + "/missing/x"); touch_error == nil {
		t.Error("a missing directory cannot be touched")
	}
}
