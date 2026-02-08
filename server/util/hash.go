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
