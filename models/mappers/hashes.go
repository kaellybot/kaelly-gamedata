package mappers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ContentHash returns the sha256 of the value as JSON. Entities exclude their own
// hash from JSON and include their labels, so a translation change is an update.
func ContentHash(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return ""
	}

	return FileHash(body)
}

// FileHash returns the sha256 of a downloaded body, hex encoded.
func FileHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
