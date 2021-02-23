package flare

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

const ProtoID = "/libp2p/flare-test"

func Proof(secret string, nonce []byte) []byte {
	secretBytes := []byte(secret)
	blob := append(secretBytes, nonce...)
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

func Verify(secret string, nonce, proof []byte) bool {
	expected := Proof(secret, nonce)
	return bytes.Compare(expected, proof) == 0
}
