package base_value_objects_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
)

func TestIdentifierNeverTrimsOrChangesTheText(test *testing.T) {
	identifier, construction_error := base_value_objects.NewIdentifier("  padded name ")
	if construction_error != nil || identifier.String() != "  padded name " {
		test.Fatalf("got %q, %v", identifier, construction_error)
	}
}

func TestIdentifierReportsTheRuleThatWasBroken(test *testing.T) {
	cases := []struct {
		description string
		text        string
		sentinels   []error
	}{
		{"empty", "", []error{base_value_objects.ErrIdentifierBlank}},
		{"spaces", "   ", []error{base_value_objects.ErrIdentifierBlank}},
		{"a newline inside", "a\nb", []error{base_value_objects.ErrIdentifierHasControlCharacters}},
		{"a tab is blank and a control character", "\t", []error{base_value_objects.ErrIdentifierBlank, base_value_objects.ErrIdentifierHasControlCharacters}},
		{"not UTF-8", "a\xffb", []error{base_value_objects.ErrIdentifierNotUTF8}},
	}
	for _, current := range cases {
		validation_error := base_value_objects.ValidateIdentifier(current.text)
		if validation_error == nil {
			test.Errorf("%s: accepted", current.description)
		}
		for _, sentinel := range current.sentinels {
			if !errors.Is(validation_error, sentinel) {
				test.Errorf("%s: %v does not hold %v", current.description, validation_error, sentinel)
			}
		}
		if _, construction_error := base_value_objects.NewIdentifier(current.text); construction_error == nil {
			test.Errorf("%s: NewIdentifier accepted", current.description)
		}
	}
}

func TestHexDigitsReportsTheRuleThatWasBroken(test *testing.T) {
	cases := []struct {
		description string
		value       string
		width       int
		sentinels   []error
		absent      []error
	}{
		{"a non-hexadecimal digit", "zz", 2, []error{base_value_objects.ErrHexDigitsInvalid}, []error{base_value_objects.ErrHexDigitsWrongWidth}},
		{"uppercase", "AB", 2, []error{base_value_objects.ErrHexDigitsInvalid}, nil},
		{"too short", "ab", 4, []error{base_value_objects.ErrHexDigitsWrongWidth}, []error{base_value_objects.ErrHexDigitsInvalid}},
		{"both rules", "zzz", 2, []error{base_value_objects.ErrHexDigitsInvalid, base_value_objects.ErrHexDigitsWrongWidth}, nil},
		{"a width of zero", "", 0, []error{base_value_objects.ErrHexDigitsWrongWidth}, nil},
		{"a negative width", "ab", -2, []error{base_value_objects.ErrHexDigitsWrongWidth}, nil},
	}
	for _, current := range cases {
		validation_error := base_value_objects.ValidateHexDigits(current.value, current.width)
		for _, sentinel := range current.sentinels {
			if !errors.Is(validation_error, sentinel) {
				test.Errorf("%s: %v does not hold %v", current.description, validation_error, sentinel)
			}
		}
		for _, sentinel := range current.absent {
			if errors.Is(validation_error, sentinel) {
				test.Errorf("%s: %v holds %v", current.description, validation_error, sentinel)
			}
		}
		if _, construction_error := base_value_objects.NewHexDigits(current.value, current.width); construction_error == nil {
			test.Errorf("%s: NewHexDigits accepted", current.description)
		}
	}
}

func TestJoinErrorsKeepsEveryErrorAndIgnoresNil(test *testing.T) {
	if base_value_objects.JoinErrors(nil, nil) != nil || base_value_objects.JoinErrors() != nil {
		test.Fatal("joining nothing must give nil")
	}
	joined := base_value_objects.JoinErrors(nil, base_value_objects.ErrHexDigitsInvalid, base_value_objects.ErrIdentifierBlank)
	if !errors.Is(joined, base_value_objects.ErrHexDigitsInvalid) || !errors.Is(joined, base_value_objects.ErrIdentifierBlank) {
		test.Fatalf("a joined error lost a cause: %v", joined)
	}
}
