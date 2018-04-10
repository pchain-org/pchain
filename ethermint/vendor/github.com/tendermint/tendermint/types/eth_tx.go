package types

import (
	//"github.com/ethereum/go-ethereum/common"
	"bytes"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

var TDM_NONCE = uint64(0xffffff)

func NewEthTransaction(sender string, etd *ethTypes.ExtendTxData) (Tx, error){

	tx := ethTypes.NewTransactionEx(TDM_NONCE, nil, nil, nil, nil, []byte{}, etd)
	buf := new(bytes.Buffer)
	if err := tx.EncodeRLP(buf); err != nil {
		return nil, nil
	}

	return buf.Bytes(), nil
}
