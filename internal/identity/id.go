package identity

import (
	"crypto/rand"
	"encoding/hex"
)

func New() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func Valid(id string) bool { b, err := hex.DecodeString(id); return err == nil && len(b) == 12 }
