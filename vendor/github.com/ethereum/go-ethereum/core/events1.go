package core

import (
	"github.com/ethereum/go-ethereum/common"
)


type CreateChildChainEvent struct{
	From common.Address
	ChainId string
}

