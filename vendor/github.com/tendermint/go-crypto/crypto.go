package crypto

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
)

func CheckConsensusPubKey(from common.Address, consensusPubkey, signature []byte) error {
	if len(consensusPubkey) != 128 {
		return errors.New("invalid consensus public key")
	}

	if len(signature) != 64 {
		return errors.New("invalid signature")
	}

	// Get BLS Public Key
	var blsPK BLSPubKey
	copy(blsPK[:], consensusPubkey)
	// Get BLS Signature
	blsSign := BLSSignature(signature)
	// Verify the Signature
	success := blsPK.VerifyBytes(from.Bytes(), blsSign)
	if !success {
		return errors.New("consensus public key signature verification failed")
	}
	return nil
}

