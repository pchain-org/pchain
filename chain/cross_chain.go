package chain

import (
	"sync"
	"github.com/ethereum/go-ethereum/common"
	"fmt"
	"github.com/ethereum/go-ethereum/event"
	dbm "github.com/tendermint/go-db"
	"github.com/pkg/errors"
)

type CrossChainHelper struct {
	mtx  sync.Mutex
	typeMut *event.TypeMux
	chainInfoDB dbm.DB
}

func (cch *CrossChainHelper) GetMutex() *sync.Mutex {
	return &cch.mtx
}

func (cch *CrossChainHelper) GetTypeMutex() *event.TypeMux {
	return cch.typeMut
}

//TODO multi-chain
func (cch *CrossChainHelper) CanCreateChildChain(from common.Address, chainId string) error {

	fmt.Printf("cch CanCreateChildChain called")

	//check if "chainId" has been created/registered
	ci := GetChainInfo(cch.chainInfoDB, chainId)
	if ci != nil {
		return errors.New(fmt.Sprint("chain %s does exist, can't create again", chainId))
	}

	//check if "from" is a legal validator in main chain

	return nil
}

func (cch *CrossChainHelper) CreateChildChain(from common.Address, chainId string) error {

	fmt.Printf("cch CreateChildChain called\n")

	//write the child chain info to "multi-chain" db
	ci := GetChainInfo(cch.chainInfoDB, chainId)
	if ci != nil {
		fmt.Printf("chain %s does exist, can't create again\n", chainId)
		//return nil, because this could be executed for the same TX!!!
		return nil
	}

	ci = &ChainInfo {owner: from, chainId: chainId}

	SaveChainInfo(cch.chainInfoDB, ci)

	return nil
}
