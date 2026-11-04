package system_random_test

import (
	"math"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/dependencies"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/system_random"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/outbound/shared/system_random/external"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/tests/fakes"
)

var (
	_ outbound.RandomPort = (*system_random.SystemRandom)(nil)
	_ outbound.RandomPort = (*fakes.FakeRandom)(nil)
)

func Test_the_system_source_gives_numbers_in_the_range_that_are_not_all_the_same(t *testing.T) {
	source := system_random.NewSystemRandom()
	seen := map[float64]bool{}
	for index := 0; index < 1000; index++ {
		number := source.Float64()
		if number < 0 || number > 1 || math.IsNaN(number) {
			t.Fatalf("out of range: %v", number)
		}
		seen[number] = true
	}
	if len(seen) < 900 {
		t.Fatalf("the source repeats itself: %d different numbers in 1000", len(seen))
	}
}

func Test_the_default_source_of_the_composition_root_is_the_system_one(t *testing.T) {
	if number := dependencies.NewSystemRandom().Float64(); number < 0 || number > 1 {
		t.Fatalf("out of range: %v", number)
	}
}

func Test_the_external_read_is_in_the_unit_range(t *testing.T) {
	for index := 0; index < 100; index++ {
		if number := external.ReadFloat64(); number < 0 || number >= 1 {
			t.Fatalf("out of range: %v", number)
		}
	}
}

func Test_a_number_outside_the_range_is_cut_to_it(t *testing.T) {
	cases := []struct {
		given    float64
		expected float64
	}{
		{-0.5, 0}, {1.5, 1}, {math.NaN(), 0}, {math.Inf(1), 1}, {math.Inf(-1), 0}, {0.25, 0.25}, {0, 0}, {1, 1},
	}
	for _, test_case := range cases {
		source := system_random.NewSystemRandomWithFunction(func() float64 { return test_case.given })
		if got := source.Float64(); got != test_case.expected {
			t.Errorf("%v: got %v, expected %v", test_case.given, got, test_case.expected)
		}
	}
}

func Test_the_fake_source_answers_in_order_repeats_the_last_and_counts(t *testing.T) {
	fake := fakes.NewFakeRandom(0.1, 0.9)
	got := []float64{fake.Float64(), fake.Float64(), fake.Float64()}
	if got[0] != 0.1 || got[1] != 0.9 || got[2] != 0.9 || fake.Calls() != 3 {
		t.Fatalf("got %v with %d calls", got, fake.Calls())
	}
	if fakes.NewFakeRandom().Float64() != 0.5 {
		t.Fatal("a fake with no numbers answers 0.5")
	}
}
