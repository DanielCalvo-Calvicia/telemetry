package masking_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

func new_masker_with(t *testing.T, options masking.MaskerOptions) masking.SensitiveDataMasker {
	t.Helper()
	masker, err := masking.NewSensitiveDataMasker(options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return masker
}

// =============================================================================
// FIELD NAMES: REPLACE OR ADD
// =============================================================================

func Test_a_list_of_field_names_replaces_the_default_list_but_the_detectors_stay_on(t *testing.T) {
	masker := new_masker_with(t, masking.MaskerOptions{FieldNames: []string{"pin"}})

	if masker.MaskValue("password", "plain") != "plain" || masker.MaskValue("token", "t") != "t" {
		t.Error("the default names must not apply when the list is replaced")
	}
	if masker.MaskValue("pin", "1234") != domain.MaskedText || masker.MaskValue("home_pin", "1234") != domain.MaskedText {
		t.Error("a name of the new list must be hidden, also as a suffix")
	}
	if masker.MaskText("password=hunter2") != "password=*****MASKED*****" {
		t.Error("the detectors of text must stay on when the list of names is replaced")
	}
	if masker.MaskValue("detail", fake_access_key_id) != domain.MaskedText {
		t.Error("a secret by what it is must stay hidden when the list of names is replaced")
	}
}

func Test_an_empty_list_of_field_names_replaces_the_default_list_with_nothing(t *testing.T) {
	masker := new_masker_with(t, masking.MaskerOptions{FieldNames: []string{}})

	if masker.MaskValue("password", "plain") != "plain" {
		t.Error("an empty, non-nil list must replace the default list")
	}
}

func Test_a_nil_list_of_field_names_keeps_the_default_list(t *testing.T) {
	masker := new_masker_with(t, masking.MaskerOptions{FieldNames: nil})

	if masker.MaskValue("password", "plain") != domain.MaskedText {
		t.Error("a nil list must keep the default list")
	}
}

func Test_extra_field_names_are_added_to_the_active_list_and_are_normalized(t *testing.T) {
	default_masker := new_masker_with(t, masking.MaskerOptions{ExtraFieldNames: []string{"Social-Security.Number", ""}})
	replaced_masker := new_masker_with(t, masking.MaskerOptions{FieldNames: []string{"pin"}, ExtraFieldNames: []string{"ssn"}})

	if default_masker.MaskValue("socialSecurityNumber", "1") != domain.MaskedText || default_masker.MaskValue("password", "p") != domain.MaskedText {
		t.Error("an extra name must be added to the default list, whatever its writing")
	}
	if default_masker.MaskValue("note", "n") != "n" {
		t.Error("an empty extra name must not hide everything")
	}
	if replaced_masker.MaskValue("ssn", "1") != domain.MaskedText || replaced_masker.MaskValue("pin", "1") != domain.MaskedText || replaced_masker.MaskValue("password", "p") != "p" {
		t.Error("an extra name must be added to the replaced list")
	}
}

// =============================================================================
// CLASSIFICATION: DROP, REDACT, ALLOW
// =============================================================================

func new_classified_masker(t *testing.T) masking.SensitiveDataMasker {
	t.Helper()
	return new_masker_with(t, masking.MaskerOptions{
		FieldClasses: map[string]string{"PatientName": "pii", "email": "contact", "Body": "internal", "mystery": "unknown_class", "": "ignored"},
		ClassRules:   map[string]string{"pii": domain.MaskRuleDrop, "contact": domain.MaskRuleRedact, "internal": domain.MaskRuleAllow},
	})
}

func Test_the_rule_drop_removes_the_field_at_any_depth_and_the_name_is_normalized(t *testing.T) {
	masker := new_classified_masker(t)

	masked_map := masker.MaskMap(map[string]any{
		"patient_name": "Ann",
		"nested":       map[string]any{"patient-name": nil, "keep": 1, "rows": []any{map[string]any{"PatientName": "x", "id": 2}}},
		"through_json": &fake_credentials{User: "u"},
	})

	if _, still_there := masked_map["patient_name"]; still_there {
		t.Error("the field of a class with the rule drop must be removed")
	}
	nested := masked_map["nested"].(map[string]any)
	if _, still_there := nested["patient-name"]; still_there || nested["keep"] != 1 {
		t.Errorf("nested = %v", nested)
	}
	if row := nested["rows"].([]any)[0].(map[string]any); len(row) != 1 || row["id"] != 2 {
		t.Errorf("row = %v", row)
	}
}

func Test_a_single_value_of_a_dropped_field_is_masked_never_returned_as_it_is(t *testing.T) {
	masker := new_classified_masker(t)

	if masked_value := masker.MaskValue("patient_name", "password=hunter2"); masked_value != "password=*****MASKED*****" {
		t.Errorf("MaskValue() = %v", masked_value)
	}
}

func Test_the_rule_redact_replaces_the_value_whatever_it_holds(t *testing.T) {
	masker := new_classified_masker(t)

	for _, value := range []any{"a@b.c", 5, map[string]any{"k": "v"}, []any{"x"}} {
		if masked_value := masker.MaskValue("email", value); masked_value != domain.MaskedText {
			t.Errorf("MaskValue(%v) = %v, want the mask", value, masked_value)
		}
	}
	if masker.MaskValue("email", nil) != nil {
		t.Error("a nil value stays nil")
	}
}

func Test_the_rule_allow_skips_the_name_rule_only(t *testing.T) {
	masker := new_classified_masker(t)

	if masker.MaskValue("body", "visible text") != "visible text" {
		t.Error("the rule allow must skip the rule of the sensitive names")
	}
	if masker.MaskValue("body", "password=hunter2") != "password=*****MASKED*****" {
		t.Error("the rule allow must not skip the detectors of text")
	}
	walked := masker.MaskValue("body", map[string]any{"token": "t", "n": 1}).(map[string]any)
	if walked["token"] != domain.MaskedText || walked["n"] != 1 {
		t.Errorf("the rule allow must still walk the maps, got %v", walked)
	}
	if masker.MaskValue("message_body", "x") != domain.MaskedText {
		t.Error("another name that ends with the same word has no class and keeps the name rule")
	}
}

func Test_a_class_without_a_rule_is_redacted(t *testing.T) {
	masker := new_classified_masker(t)

	if masker.MaskValue("mystery", "v") != domain.MaskedText {
		t.Error("a class without a rule must be redacted")
	}
}

func Test_two_names_that_read_the_same_never_depend_on_the_order_of_the_map(t *testing.T) {
	for attempt := 0; attempt < 20; attempt++ {
		masker := new_masker_with(t, masking.MaskerOptions{
			FieldClasses: map[string]string{"user_name": "a", "userName": "b"},
			ClassRules:   map[string]string{"a": domain.MaskRuleAllow, "b": domain.MaskRuleRedact},
		})
		if masker.MaskValue("user_name", "v") != "v" {
			t.Fatal("the name that sorts last (user_name, class a, allow) must win every time")
		}
	}
}

// =============================================================================
// ERRORS OF THE OPTIONS
// =============================================================================

func Test_a_rule_that_is_not_drop_redact_or_allow_is_a_classified_error(t *testing.T) {
	masker, err := masking.NewSensitiveDataMasker(masking.MaskerOptions{ClassRules: map[string]string{"pii": "keep"}})

	var classified *domain.ClassifiedError
	if !errors.Is(err, masking.ErrMaskRuleInvalid) || !errors.Is(err, domain.ErrMaskRuleInvalid) {
		t.Errorf("error = %v, want ErrMaskRuleInvalid", err)
	}
	if !errors.As(err, &classified) || classified.Code != value_objects.ErrorCodeConfigFieldInvalid {
		t.Errorf("error = %v, want the code config_field_invalid", err)
	}
	if masker.MaskText("password=x") != "password=x" {
		t.Error("a failed masker must be the zero masker")
	}
}

// =============================================================================
// CONCURRENCY AND IDEMPOTENCE
// =============================================================================

func Test_the_masker_is_safe_for_concurrent_use(t *testing.T) {
	masker := new_classified_masker(t)
	source_map := map[string]any{"password": "p", "note": "token=abc", "patient_name": "x", "list": []any{"Bearer abcdef123456"}}

	var wait_group sync.WaitGroup
	failures := make(chan string, 8)
	for worker := 0; worker < 8; worker++ {
		wait_group.Add(1)
		go func() {
			defer wait_group.Done()
			for round := 0; round < 200; round++ {
				if masker.MaskText("password=hunter2") != "password=*****MASKED*****" {
					failures <- "MaskText"
					return
				}
				masked_map := masker.MaskMap(source_map)
				if masked_map["password"] != domain.MaskedText || masked_map["note"] != "token=*****MASKED*****" || len(masked_map) != 3 {
					failures <- "MaskMap"
					return
				}
			}
		}()
	}
	wait_group.Wait()
	close(failures)

	for failure := range failures {
		t.Errorf("a concurrent call of %s gave a wrong result", failure)
	}
}

func Test_masking_a_masked_map_changes_nothing(t *testing.T) {
	masker := new_classified_masker(t)
	source_map := map[string]any{"password": "p", "note": "postgres://u:p@h/db token=abc", "email": "a@b.c", "n": 1.5, "list": []any{fake_jwt}}

	once := masker.MaskMap(source_map)
	twice := masker.MaskMap(once)

	if len(once) != len(twice) {
		t.Fatalf("once = %v, twice = %v", once, twice)
	}
	for key, value := range once {
		if key != "list" && value != twice[key] {
			t.Errorf("key %s: once = %v, twice = %v", key, value, twice[key])
		}
	}
	if twice["list"].([]any)[0] != domain.MaskedText {
		t.Errorf("list = %v", twice["list"])
	}
}
