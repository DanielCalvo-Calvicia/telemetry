package canonical_json

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// MaxExactInteger is the largest integer that prints exactly: 2^53 - 1 (9007199254740991). An integer beyond it, in
// either direction, is rejected with the code limit_exceeded. It is the one place to change if the maintainer decides
// to serialize such integers as strings instead (T-02.5, open question).
const MaxExactInteger int64 = 1<<53 - 1

// FormatInt prints an integer in the canonical form. It rejects an integer beyond +-MaxExactInteger.
func FormatInt(value int64) (string, error) {
	if value > MaxExactInteger || value < -MaxExactInteger {
		return "", domain.NewClassifiedError(
			value_objects.ErrorCodeLimitExceeded, domain.ErrNumberOutOfRange,
			fmt.Errorf("the integer %d is beyond +-%d", value, MaxExactInteger))
	}
	return strconv.FormatInt(value, 10), nil
}

// FormatFloat prints a double in the ECMAScript Number-to-string form that RFC 8785 requires: the shortest digits
// that round-trip, an exponent form below 1e-6 and from 1e21, and "0" for both zeros. NaN and the infinities are
// rejected with the code event_schema_invalid.
func FormatFloat(value float64) (string, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", domain.NewClassifiedError(
			value_objects.ErrorCodeEventSchemaInvalid, domain.ErrNumberOutOfRange,
			errors.New("NaN and the infinities have no JSON form"))
	}
	if value == 0 {
		return "0", nil
	}
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	// 'e' with precision -1 gives the shortest digits that round-trip: "d.ddde+XX" or "de+XX".
	scientific := strconv.FormatFloat(value, 'e', -1, 64)
	mantissa, exponent_text, _ := strings.Cut(scientific, "e")
	exponent, parse_error := strconv.Atoi(exponent_text)
	if parse_error != nil {
		return "", domain.NewClassifiedError(
			value_objects.ErrorCodeEventSchemaInvalid, domain.ErrNumberOutOfRange, parse_error)
	}
	digits := strings.Replace(mantissa, ".", "", 1)
	return sign + EcmaScriptForm(digits, exponent+1), nil
}

// EcmaScriptForm lays out the shortest digits of a positive number the way Number::toString of ECMAScript does.
// digits has no trailing zeros and point_position (n in the specification) is the position of the decimal point
// relative to the first digit: the value is 0.DIGITS * 10^point_position.
func EcmaScriptForm(digits string, point_position int) string {
	digit_count := len(digits)
	switch {
	case digit_count <= point_position && point_position <= 21:
		return digits + strings.Repeat("0", point_position-digit_count)
	case 0 < point_position && point_position <= 21:
		return digits[:point_position] + "." + digits[point_position:]
	case -6 < point_position && point_position <= 0:
		return "0." + strings.Repeat("0", -point_position) + digits
	}
	exponent := point_position - 1
	exponent_sign := "+"
	if exponent < 0 {
		exponent_sign = "-"
		exponent = -exponent
	}
	if digit_count == 1 {
		return digits + "e" + exponent_sign + strconv.Itoa(exponent)
	}
	return digits[:1] + "." + digits[1:] + "e" + exponent_sign + strconv.Itoa(exponent)
}
