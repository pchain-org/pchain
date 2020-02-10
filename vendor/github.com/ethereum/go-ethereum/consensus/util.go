package consensus

import (
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	"github.com/ethereum/go-ethereum/core/types"
)

func IsSelfRetrieveReward(epEntity *epoch.Epoch, chain ChainReader, header *types.Header) bool {

	curBlockNumber := header.Number.Uint64()
	curEpoch := epEntity.GetEpochByBlockNumber(curBlockNumber)

	epStartHeader := header
	if curBlockNumber != curEpoch.StartBlock {
		epStartHeader = chain.GetBlockByNumber(curEpoch.StartBlock).Header()
	}
	mainBlock := epStartHeader.Number
	if !chain.Config().IsMainChain() {
		mainBlock = epStartHeader.MainChainNumber
	}

	return chain.Config().IsSelfRetrieveReward(mainBlock)
}
