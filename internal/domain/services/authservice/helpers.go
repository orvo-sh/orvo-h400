package authservice

import (
	"crypto/sha256"
	"fmt"
)

func hashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return fmt.Sprintf("%x", h)
}
