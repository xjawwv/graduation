package room

import (
	"crypto/rand"
	"strings"
)

const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func GenerateCode(n int) (string, error) {
	if n < 4 {
		n = 6
	}
	b := make([]byte, n)
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return strings.ToUpper(string(b)), nil
}
