// Package helpers holds the pure functions of the UUID generator: no clock, no random source, no I/O.
package helpers

import "encoding/hex"

// RandomByteCount is the number of random bytes that LayoutUUIDv7 uses: 12 bits for rand_a and 62 for rand_b, taken
// from 10 bytes whose unused bits are ignored.
const RandomByteCount = 10

const (
	version_7_bits     = 0x70
	low_nibble_mask    = 0x0f
	variant_bits       = 0x80
	low_six_bits_mask  = 0x3f
	timestamp_bit_mask = (1 << 48) - 1
	uuid_byte_count    = 16
)

// LayoutUUIDv7 returns the text of a UUID of version 7 (RFC 9562) as lowercase, hyphenated 8-4-4-4-12. The first 48
// bits are the unix time in milliseconds, big-endian (only the low 48 bits of the number are kept); then the version
// 7; then 12 random bits (the low nibble of random_bytes[0] and random_bytes[1]); then the variant 10 and 62 random
// bits (the low 6 bits of random_bytes[2] and random_bytes[3] to random_bytes[9]). It is deterministic.
func LayoutUUIDv7(unix_milliseconds int64, random_bytes [RandomByteCount]byte) string {
	var raw_bytes [uuid_byte_count]byte
	timestamp := uint64(unix_milliseconds) & timestamp_bit_mask
	for index := 0; index < 6; index++ {
		raw_bytes[index] = byte(timestamp >> (8 * (5 - index)))
	}
	raw_bytes[6] = version_7_bits | (random_bytes[0] & low_nibble_mask)
	raw_bytes[7] = random_bytes[1]
	raw_bytes[8] = variant_bits | (random_bytes[2] & low_six_bits_mask)
	copy(raw_bytes[9:], random_bytes[3:])

	hex_text := hex.EncodeToString(raw_bytes[:])
	return hex_text[0:8] + "-" + hex_text[8:12] + "-" + hex_text[12:16] + "-" + hex_text[16:20] + "-" + hex_text[20:32]
}
