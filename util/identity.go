package util

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/libp2p/go-libp2p-core/crypto"
)

func LoadIdentity(idPath string) (crypto.PrivKey, error) {
	if _, err := os.Stat(idPath); err == nil {
		return readIdentity(idPath)
	} else if os.IsNotExist(err) {
		fmt.Printf("Generating peer identity in %s\n", idPath)
		return generatePersistentIdentity(idPath)
	} else {
		return nil, err
	}
}

func readIdentity(path string) (crypto.PrivKey, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(bytes)
}

func generatePersistentIdentity(path string) (crypto.PrivKey, error) {
	privk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		return nil, err
	}

	bytes, err := crypto.MarshalPrivateKey(privk)
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(path, bytes, 0400)

	return privk, err
}

func GenerateIdentity() (crypto.PrivKey, error) {
	privk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	return privk, err
}
