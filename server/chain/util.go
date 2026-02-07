package chain

import (
	"encoding/base64"
)

// encodeBase64 encodes bytes to a base64 string (standard encoding).
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
