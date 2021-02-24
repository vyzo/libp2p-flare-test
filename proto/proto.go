package proto

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

const ProtoID = "/libp2p/flare-test/presence"

func Proof(secret string, salt, nonce []byte) []byte {
	secretBytes := []byte(secret)
	blob := make([]byte, len(secretBytes)+len(nonce)+len(salt))
	n := copy(blob, salt)
	n += copy(blob[n:], secretBytes)
	copy(blob[n:], nonce)
	hash := sha256.Sum256(blob)
	return hash[:]
}

func Nonce() ([]byte, error) {
	nonce := make([]byte, 32)
	n, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	if n != 32 {
		return nil, fmt.Errorf("not enough random bytes; asked for 32, got %d", n)
	}
	return nonce, nil
}

func Verify(secret string, salt, nonce, proof []byte) bool {
	expected := Proof(secret, salt, nonce)
	return bytes.Compare(expected, proof) == 0
}
