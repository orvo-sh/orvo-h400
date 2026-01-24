package util

import (
	"crypto/rand"
	"encoding/base64"
)

func GenerateRandomString(length ...int) string {
	l := 32
	if len(length) > 0 {
		l = length[0]
	}
	bytes := make([]byte, l)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}
