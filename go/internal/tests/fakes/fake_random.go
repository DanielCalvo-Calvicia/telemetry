package fakes

import "sync"

// FakeRandom is a fake of outbound.RandomPort. It returns the numbers that were set, in order, and the last one again
// when they are used up; with none set it returns 0.5. It counts the calls and guards its state with a mutex.
type FakeRandom struct {
	mutex   sync.Mutex
	numbers []float64
	calls   int
}

// NewFakeRandom returns a FakeRandom that answers the numbers in order (the last one repeats).
func NewFakeRandom(numbers ...float64) *FakeRandom {
	return &FakeRandom{numbers: numbers}
}

// Float64 returns the next number that was set.
func (fake_random *FakeRandom) Float64() float64 {
	fake_random.mutex.Lock()
	defer fake_random.mutex.Unlock()
	fake_random.calls++
	if len(fake_random.numbers) == 0 {
		return 0.5
	}
	index := min(fake_random.calls-1, len(fake_random.numbers)-1)
	return fake_random.numbers[index]
}

// Calls returns the number of times Float64 was called.
func (fake_random *FakeRandom) Calls() int {
	fake_random.mutex.Lock()
	defer fake_random.mutex.Unlock()
	return fake_random.calls
}
