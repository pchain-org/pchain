package types

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sync"

	"bls"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
)

type PrivValidator struct {
	// PChain Account Address, same as Ethereum Address Format
	Address common.Address `json:"address"`
	// PChain Consensus Public Key, in BLS format
	PubKey crypto.PubKey `json:"consensus_pub_key"`
	// PChain Consensus Private Key, in BLS format
	// PrivKey should be empty if a Signer other than the default is being used.
	PrivKey crypto.PrivKey `json:"consensus_priv_key"`

	Signer `json:"-"`

	// For persistence.
	// Overloaded for testing.
	filePath string
	mtx      sync.Mutex
}

// This is used to sign votes.
// It is the caller's duty to verify the msg before calling Sign,
// eg. to avoid double signing.
// Currently, the only callers are SignVote and SignProposal
type Signer interface {
	Sign(msg []byte) crypto.Signature
}

// Implements Signer
type DefaultSigner struct {
	priv crypto.PrivKey
}

func NewDefaultSigner(priv crypto.PrivKey) *DefaultSigner {
	return &DefaultSigner{priv: priv}
}

// Implements Signer
func (ds *DefaultSigner) Sign(msg []byte) crypto.Signature {
	return ds.priv.Sign(msg)
}

func GenPrivValidatorKey(address common.Address) *PrivValidator {

	keyPair := bls.GenerateKey()
	var blsPrivKey crypto.BLSPrivKey
	copy(blsPrivKey[:], keyPair.Private().Marshal())

	blsPubKey := blsPrivKey.PubKey()

	return &PrivValidator{
		Address: address,
		PubKey:  blsPubKey,
		PrivKey: blsPrivKey,

		filePath: "",
		Signer:   NewDefaultSigner(blsPrivKey),
	}
}

func LoadPrivValidator(filePath string) *PrivValidator {
	privValJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(err.Error())
	}
	privVal := wire.ReadJSON(&PrivValidator{}, privValJSONBytes, &err).(*PrivValidator)
	if err != nil {
		Exit(Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}
	privVal.filePath = filePath
	privVal.Signer = NewDefaultSigner(privVal.PrivKey)
	return privVal
}

func (pv *PrivValidator) SetFile(filePath string) {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()

	pv.filePath = filePath
}

func (pv *PrivValidator) Save() {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()

	pv.save()
}

func (pv *PrivValidator) save() {
	if pv.filePath == "" {
		PanicSanity("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes := wire.JSONBytesPretty(pv)
	err := WriteFileAtomic(pv.filePath, jsonBytes, 0600)
	if err != nil {
		// `@; BOOM!!!
		PanicCrisis(err)
	}
}

func (pv *PrivValidator) GetAddress() []byte {
	return pv.Address.Bytes()
}

func (pv *PrivValidator) GetPubKey() crypto.PubKey {
	return pv.PubKey
}

func (pv *PrivValidator) SignVote(chainID string, vote *Vote) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()

	signature := pv.Sign(SignBytes(chainID, vote))
	vote.Signature = signature
	return nil
}

func (pv *PrivValidator) SignProposal(chainID string, proposal *Proposal) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()

	signature := pv.Sign(SignBytes(chainID, proposal))
	proposal.Signature = signature
	return nil
}

func (pv *PrivValidator) String() string {
	return fmt.Sprintf("PrivValidator{%X}", pv.Address)
}

//-------------------------------------

type PrivValidatorsByAddress []*PrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].Address[:], pvs[j].Address[:]) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}
