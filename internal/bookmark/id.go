package bookmark

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "bookmark"
	}
	return hex.EncodeToString(b[:])
}
