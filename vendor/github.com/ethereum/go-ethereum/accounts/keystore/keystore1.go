package keystore

import (
	"io"
	"github.com/ethereum/go-ethereum/common"
)

type KeyStorePassphrase struct {
	Ks keyStorePassphrase
}


func NewKeyStoreByTenermint(keydir string, scryptN, scryptP int) *KeyStorePassphrase {
	return &KeyStorePassphrase{keyStorePassphrase{keydir, scryptN, scryptP}}
}

func (ks KeyStorePassphrase) StoreKey(filename string, key *Key, auth string) error {
	return (ks.Ks).StoreKey(filename, key, auth)
}

func NewKey(rand io.Reader) (*Key, error) {
	return newKey(rand)
}

func KeyFileName(keyAddr common.Address) string {
	return keyFileName(keyAddr)
}
