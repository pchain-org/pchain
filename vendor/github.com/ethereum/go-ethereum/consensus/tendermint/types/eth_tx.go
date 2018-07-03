package types

import (
	//"github.com/ethereum/go-ethereum/common"
	"bytes"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

var TDM_NONCE = uint64(0xffffff)
var INNER_GAS_LIMIT = uint64(0)

func NewEthTransaction(sender string, etd *ethTypes.ExtendTxData) (Tx, error){

	tx := ethTypes.NewTransactionEx(TDM_NONCE, nil, nil, INNER_GAS_LIMIT, nil, []byte{}, etd)
	buf := new(bytes.Buffer)
	if err := tx.EncodeRLP(buf); err != nil {
		return nil, nil
	}

	return buf.Bytes(), nil
}
