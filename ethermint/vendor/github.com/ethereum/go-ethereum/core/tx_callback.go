package core

import (

	"errors"
	st "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

type EtdValidateCb func(tx *types.Transaction, state *st.StateDB) error
type EtdApplyCb func(tx *types.Transaction, state *st.StateDB) error

var validateCbMap map[string]EtdValidateCb = make(map[string]EtdValidateCb)
var applyCbMap    map[string]EtdApplyCb = make(map[string]EtdApplyCb)

func RegisterValidateCb(name string, validateCb EtdValidateCb) error {

	//fmt.Printf("RegisterValidateCb (name, validateCb) = (%s, %v)\n", name, validateCb)
	_, ok := validateCbMap[name]
	if ok {
		//fmt.Printf("RegisterValidateCb return (%v, %v)\n", cb, ok)
		return errors.New("the name has registered in ValidateCbMap")
	}

	validateCbMap[name] = validateCb
	return nil
}

func GetValidateCb(name string) EtdValidateCb {

	cb, ok := validateCbMap[name]
	if ok {
		return cb
	}

	return nil
}

func RegisterApplyCb(name string, applyCb EtdApplyCb) error {

	_, ok := applyCbMap[name]
	if ok {
		return errors.New("the name has registered in ValidateCbMap")
	}

	applyCbMap[name] = applyCb

	return nil
}

func GetApplyCb(name string) EtdApplyCb {

	cb, ok := applyCbMap[name]
	if ok {
		return cb
	}

	return nil
}

