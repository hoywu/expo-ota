package models

import "encoding/hex"

// ByteaHex encodes raw bytes as the PostgreSQL hex bytea literal (`\x...`).
// lib/pq sends Go strings as text-format parameters, so bytea columns must
// be written/queried with this encoding when the model field is a string.
func ByteaHex(b []byte) string {
	return `\x` + hex.EncodeToString(b)
}
