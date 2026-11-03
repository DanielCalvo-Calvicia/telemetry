// Package helpers holds the pure part of the memory queue: a fixed-size ring of entries, with no lock.
package helpers

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// RING BUFFER
// =============================================================================

// Entry is one queued event with the time it arrived, in milliseconds of the monotonic clock.
type Entry struct {
	Envelope  value_objects.Envelope
	ArrivalMS int64
}

// RingBuffer is a first-in first-out buffer of a fixed capacity. It is not safe for concurrent use: the caller holds a
// lock. The storage is allocated once, so a push never allocates.
type RingBuffer struct {
	entries []Entry
	head    int
	size    int
}

// NewRingBuffer returns an empty RingBuffer that holds up to capacity entries. A capacity below 1 is 1.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuffer{entries: make([]Entry, capacity)}
}

// Push appends the entry and returns true, or returns false and stores nothing when the buffer is full.
func (ring *RingBuffer) Push(entry Entry) bool {
	if ring.size == len(ring.entries) {
		return false
	}
	ring.entries[(ring.head+ring.size)%len(ring.entries)] = entry
	ring.size++
	return true
}

// Take removes and returns the oldest n entries, in order: fewer when the buffer holds fewer, none when n is not
// positive. The slots it frees are cleared, so the buffer holds no reference to a taken event.
func (ring *RingBuffer) Take(n int) []Entry {
	if n > ring.size {
		n = ring.size
	}
	if n <= 0 {
		return nil
	}
	taken := make([]Entry, n)
	for index := 0; index < n; index++ {
		slot := (ring.head + index) % len(ring.entries)
		taken[index] = ring.entries[slot]
		ring.entries[slot] = Entry{}
	}
	ring.head = (ring.head + n) % len(ring.entries)
	ring.size -= n
	return taken
}

// Len returns the number of entries held.
func (ring *RingBuffer) Len() int {
	return ring.size
}

// Oldest returns the arrival time of the oldest entry, and false when the buffer is empty.
func (ring *RingBuffer) Oldest() (int64, bool) {
	if ring.size == 0 {
		return 0, false
	}
	return ring.entries[ring.head].ArrivalMS, true
}
