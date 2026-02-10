package util

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken computes the SHA-256 hex digest of a secret token.
// Used for API keys and session tokens so that only the hash is stored in the DB.
// SHA-256 is sufficient here because the inputs have high entropy (32 random bytes).
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// HashContent computes the SHA-256 hex digest of fragment content.
// This serves as a public "fingerprint" for transparent verification:
// anyone with the original text can recompute the hash to prove data integrity,
// but the hash alone cannot reveal the original content.
func HashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
