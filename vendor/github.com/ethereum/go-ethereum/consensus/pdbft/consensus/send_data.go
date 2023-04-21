package consensus


import (
	"github.com/ethereum/go-ethereum/common"
	consss "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pdbft/epoch"
	"github.com/ethereum/go-ethereum/consensus/pdbft/types"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	pabi "github.com/pchain/abi"
	tmdcrypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"

	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

var sleepDuration = time.Millisecond * 200


var sendTxVars struct {
	account      common.Address
	signer       ethTypes.Signer
	prv          *ecdsa.PrivateKey
	ctx          context.Context
	nonceCache   uint64
}

func (cs *ConsensusState) sendDataRoutine() {
	cs.wg.Add(1)
	defer func() {
		cs.wg.Done()
		cs.logger.Infof("ConsensusState done senddata routine")
	}()

	cs.sendData.initialize()

	for cs.IsRunning() {

		//let's schedule the sending operation
		cs.sendData.ProcessOneRound()

		time.Sleep(sleepDuration)
	}
}

func (sc *ConsensusState) saveBlockToMainChain(blockNumber uint64, version int) error {
	if sc.sendData.hasItem(blockNumber) {
		return nil
	}
	item := &SDItem{
		isParent: true,
		BlockNumber: blockNumber,
		EpochNumber: -1,
	}
	return sc.sendData.addOrUpdateDataItem(item)
}

var (
	indexKeyFmt     = "Data-%x" //%x is blockNumber
	indexKeyPrefix  = []byte("Data-")
	indexValue      = []byte("~o~") //position occupation
	contentKeyFmt   = "Content-%x"  //%x is the number of which block to send
	contentKeyPrefix = []byte("Content-")

	ExceedTxCount   = errors.New("exceed the tx count")
)


type SDItem struct{

	//only parent item would be stored in db
	isParent        bool
	//for parent, key is block number string, for child, key is tx3beginindex
	children        map[string]*SDItem
	parent          *SDItem
	proofDataBytes  []byte
	tx3Hashes       []common.Hash

	BlockNumber     uint64
	EpochNumber     int //less than zero means no epoch information
	//when the count of tx3es is too many, split them into different sd2vc txes
	//for example there is 230 tx3es, send them with different 3 sd2vc txes which may contains tx3
	//like (Tx3BeginIndex, Tx3Count) = (0, 100), (100, 100), (200, 30]
	Tx3BeginIndex   uint64
	Tx3Count        uint64 //equcal zero means no tx3
	SentTimes       int
	Hash            common.Hash //if sd2mc tx has sent, here is the hash
}

type SendData struct {
	DB              ethdb.Database
	items           map[string]*SDItem //string should be "blockNumber" or "blockNumber+tx3BeginIndex+tx3EndIndex"
	dbMtx           sync.Mutex

	runMtx			sync.Mutex

	CS              *ConsensusState
}

func NewSendData(db ethdb.Database) *SendData{
	sendData := &SendData{
		DB: db,
		items: make(map[string]*SDItem),
	}
	return sendData
}

func calcMapKey(key uint64) string {
	return fmt.Sprintf("%x", key)
}

func calcDBKey(keyFmt string, blockNumber uint64) []byte {
	return []byte(fmt.Sprintf(keyFmt, blockNumber))
}

func (sd *SendData) PrivValidator() *types.PrivValidator {
	return sd.CS.GetPrivValidator().(*types.PrivValidator)
}

func (sd *SendData) CCH() core.CrossChainHelper {
	return sd.CS.cch
}

func (sd *SendData) ChainReader() consss.ChainReader {
	return sd.CS.GetChainReader()
}

func (sd *SendData) Logger() log.Logger {
	return sd.CS.logger
}

func (sd *SendData) hasItem(blockNumber uint64) bool {
	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	_, exist:= sd.items[calcMapKey(blockNumber)]
	return exist
}

func (sd *SendData) addOrUpdateDataItem(item *SDItem) error {

	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	if item.isParent {
		sd.items[calcMapKey(item.BlockNumber)] = item

		err := sd.DB.Put(calcDBKey(indexKeyFmt, item.BlockNumber), indexValue)
		if err != nil {
			return err
		}

		contentBytes := wire.BinaryBytes(*item)
		err = sd.DB.Put(calcDBKey(contentKeyFmt, item.BlockNumber), contentBytes)
		if err != nil {
			return nil
		}

	} else {
		parentItem, exist := sd.items[calcMapKey(item.BlockNumber)]
		if !exist {
			err := fmt.Errorf("parent item should exist, need fix")
			sd.Logger().Crit("err: %v", err)
			return err
		}
		item.parent = parentItem
		if parentItem.children == nil {
			parentItem.children = make(map[string]*SDItem)
		}
		parentItem.children[calcMapKey(item.Tx3BeginIndex)] = item
	}

	return nil
}

//only parent item needs db operation
func (sd *SendData) deleteDataItem (isParent bool, blockNumber, tx3BeginIndex uint64) error {

	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	sdItem := sd.items[calcMapKey(blockNumber)]
	deleteParent := isParent
	if !deleteParent {
		delete(sdItem.children, calcMapKey(tx3BeginIndex))
		if len(sdItem.children) == 0 {
			deleteParent = true
		}
	}

	if deleteParent {
		err := sd.DB.Delete(calcDBKey(indexKeyFmt, blockNumber))
		if err != nil {
			return err
		}

		err = sd.DB.Delete(calcDBKey(contentKeyFmt, blockNumber))
		if err != nil {
			return err
		}

		sdItem.children = nil
		delete(sd.items, calcMapKey(blockNumber))
	}

	return nil
}

func (sd *SendData) initialize() error {

	//initialize sendTxVars
	prv, err := crypto.ToECDSA(sd.PrivValidator().PrivKey.(tmdcrypto.BLSPrivKey).Bytes())
	if err != nil {
		sd.Logger().Error("initialize: failed to get PrivateKey", "err", err)
		return err
	}
	mainChainId := sd.CCH().GetMainChainId()
	digest := crypto.Keccak256([]byte(mainChainId))

	sendTxVars.prv = prv
	sendTxVars.account = crypto.PubkeyToAddress(sendTxVars.prv.PublicKey)
	sendTxVars.signer = ethTypes.LatestSignerForChainID(new(big.Int).SetBytes(digest[:]))
	sendTxVars.ctx, _ = context.WithTimeout(context.Background(), 3*time.Second)
	sendTxVars.nonceCache = 0

	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()
	
	//load all items waiting to be handled
	iter := sd.DB.NewIteratorWithPrefix(indexKeyPrefix)
	for iter.Next() {
		key := iter.Key()

		if !bytes.HasPrefix(key, indexKeyPrefix) {
			break
		}

		blockNumber := uint64(0)
		blockNumberStr := string(key)[len(indexKeyPrefix):] //remove "Data-"
		fmt.Sscanf(blockNumberStr, "%x", &blockNumber)
		insContentKey := calcDBKey(contentKeyFmt, blockNumber)
		itemData, err := sd.DB.Get(insContentKey)
		if len(itemData)==0 || err != nil {
			continue
		}

		oneItem := &SDItem{}
		err = wire.ReadBinaryBytes(itemData, oneItem)
		if err != nil {
			continue
		}

		sd.items[blockNumberStr] = oneItem
	}

	sd.Logger().Infof("there are %v sditems loaded\n", len(sd.items))

	return nil
}

func (sd *SendData) ProcessOneRound()  {
	sd.PickAndCheckOneItem(false)
	sd.PickAndSendOneItem()
	return
}

func (sd *SendData) PickAndCheckOneItem(inDetail bool) {

	picked := sd.pickOneForCheck()
	if picked != nil {
		sd.Logger().Infof("PickAndCheckOneBlock: block %v is picked to check\n", picked.BlockNumber)
		sd.checkOneItem(picked, inDetail)
	}
}

func (sd *SendData) pickOneForCheck() *SDItem {
	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	picked := (*SDItem)(nil)
	for _, item := range sd.items {
		if len(item.children) != 0 {
			for _, child := range item.children {
				if !common.EmptyHash(child.Hash) {
					picked = child
					break
				}
			}
			if picked != nil {
				break
			}
		} else if !common.EmptyHash(item.Hash) {
			picked = item
			break
		}
	}

	if picked == nil {
		sd.Logger().Debug("PickAndCheckOneBlock: no block is picked to check")
	}

	return picked
}

func (sd *SendData) checkOneItem(picked *SDItem, inDetail bool) {
	sd.runMtx.Lock()
	defer sd.runMtx.Unlock()

	hash := picked.Hash
	if !common.EmptyHash(hash) {
		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
		apiBridge := sd.CCH().GetApiBridgeFromMainChain()
		isPending, err := apiBridge.GetTransactionByHash(ctx, hash)
		if err != nil {
			sd.Logger().Error("checkOneBlock: tx %x not found in main chain, send again", hash)
			picked.Hash = common.Hash{}
			sd.addOrUpdateDataItem(picked)
		} else {
			if isPending {
				sd.Logger().Infof("checkOneBlock: tx %x is pending in main chain", hash)
			} else {
				sd.Logger().Infof("checkOneBlock: tx %x was executed in main chain", hash)
				if inDetail {
					block := sd.ChainReader().GetBlockByNumber(picked.BlockNumber)
					if picked.EpochNumber >= 0 {
						sd.checkEpochInMainChain(block)
					}
					if picked.Tx3Count > 0 {
						sd.checkTx3InMainChain(block)
					}
				}
				sd.deleteDataItem(picked.isParent, picked.BlockNumber, picked.Tx3BeginIndex)
			}
		}
	}
}

func (sd *SendData) checkEpochInMainChain(block *ethTypes.Block) error {
	sd.Logger().Infof("CheckEpochInMainChain: block %v epoch was stored in main chain", block.NumberU64())
	return nil
}

func (sd *SendData) checkTx3InMainChain(block *ethTypes.Block) error {
	sd.Logger().Infof("CheckTx3InMainChain: block %v tx3 was stored in main chain", block.NumberU64())
	return nil
}

func (sd *SendData) PickAndSendOneItem() {
	picked := sd.pickOneForSend()
	if picked != nil {
		sd.sendOneItem(picked, 1)
	}
}

func (sd *SendData) pickOneForSend() *SDItem {
	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	picked := (*SDItem)(nil)
	for _, item := range sd.items {
		if len(item.children) != 0 {
			for _, child := range item.children {
				if common.EmptyHash(child.Hash) {
					picked = child
					break
				}
			}
			if picked != nil {
				break
			}
		} else if common.EmptyHash(item.Hash) {
			if picked == nil {
				picked = item
			}
			break
		}
	}

	if picked != nil {
		sd.Logger().Infof("pickOneForSend: block (%d-%d-%d) is picked to send to main chain",
			picked.BlockNumber, picked.Tx3BeginIndex, picked.Tx3Count)
	} else {
		sd.Logger().Debug("pickOneForSend: no block is picked to send")
	}

	return picked
}

type MPDRet struct {
	proofDataBytes []byte
	epochNumber    int
	tx3Count       uint64
	tx3Hashes      []common.Hash
	ended          bool
	err            error
}

func (sd *SendData) sendOneItem(picked *SDItem, version int) error {
	sd.runMtx.Lock()
	defer sd.runMtx.Unlock()

	proofDataBytes := picked.proofDataBytes
	tx3BeginIndex := uint64(0)

	if len(proofDataBytes) == 0 {
		block := sd.ChainReader().GetBlockByNumber(picked.BlockNumber)

		ended := false
		isParent := true
		for !ended {
			ret := sd.MakeProofData(block, tx3BeginIndex, version)
			if ret.err != nil {
				return ret.err
			}
			if !ret.ended && isParent {
				isParent = false
			}
			if len(ret.proofDataBytes) != 0 {
				item := &SDItem{
					isParent: isParent,
					BlockNumber: picked.BlockNumber,
					EpochNumber: ret.epochNumber,
					tx3Hashes: ret.tx3Hashes,
					Tx3BeginIndex: tx3BeginIndex,
					Tx3Count: ret.tx3Count,
					proofDataBytes: ret.proofDataBytes,
				}
				sd.addOrUpdateDataItem(item)
			}

			tx3BeginIndex += ret.tx3Count
			ended = ret.ended
		}

		if len(picked.children) != 0 {
			picked = sd.items[calcMapKey(picked.BlockNumber)].children[calcMapKey(uint64(0))]
		} else { // picked's pointer has been replaced after above addOrUpdateDataItem()
			picked = sd.items[calcMapKey(picked.BlockNumber)]
		}
		proofDataBytes = picked.proofDataBytes
	}

	hash, err := sd.SendDataToMainChain(proofDataBytes, &sendTxVars.nonceCache)
	if err == nil {
		picked.SentTimes++
		picked.Hash = hash
		sd.addOrUpdateDataItem(picked)
		if picked.Tx3Count > 0 {
			sd.Logger().Infof("sendDataToMainChain with (%v-%v-%v), first hashe is:",
				picked.BlockNumber, picked.Tx3BeginIndex, picked.Tx3Count)
			sd.Logger().Infof("%x ", picked.tx3Hashes[0])
			//for _,hash := range picked.tx3Hashes {
			//	sd.Logger().Infof("%x ", hash)
			//}
		}

		sendTxVars.nonceCache ++
	} else {
		sendTxVars.nonceCache = 0
	}
	
	return err
}

func (sd *SendData) MakeProofData(block *ethTypes.Block, tx3BeginIndex uint64, version int) (mpdRet *MPDRet) {

	mpdRet = &MPDRet{ended:true}

	tdmExtra, err := types.ExtractTendermintExtra(block.Header())
	if err != nil {
		return
	}

	if tdmExtra.EpochBytes != nil && len(tdmExtra.EpochBytes) != 0 {
		ep := epoch.FromBytes(tdmExtra.EpochBytes)
		if ep != nil {
			mpdRet.epochNumber = int(ep.Number)
		} else {
			mpdRet.epochNumber = -1
		}
	}

	proofDataBytes := []byte{}

	if version == 0 {
		proofData, err := ethTypes.NewChildChainProofData(block)
		if err != nil {
			mpdRet.err = err
			sd.Logger().Error("MakeProofData: failed to create proof data", "block", block, "err", err)
			return
		}
		proofDataBytes, err = rlp.EncodeToBytes(proofData)
		if err != nil {
			mpdRet.err = err
			sd.Logger().Error("MakeProofData: failed to encode proof data", "proof data", proofData, "err", err)
			return
		}

		mpdRet.proofDataBytes = proofDataBytes
		return
	} else {

		mpdRet.ended = false

		lastTx3Count := uint64(0)
		lastTx3Hashes := make([]common.Hash,0)
		lastProofDataBytes := []byte{}
		ended := false
		step := uint64(10)
		tx3Count := step
		for ;; {
			proofData, tx3Hashes, err := NewChildChainProofDataV1(block, tx3BeginIndex, tx3Count)
			if err == ExceedTxCount {
				mpdRet.ended = true
				return
			} else if err != nil {
				mpdRet.err = err
				sd.Logger().Error("MakeProofData: failed to create proof data", "block", block, "err", err)
				return
			}
			proofDataBytes, err = rlp.EncodeToBytes(proofData)
			if err != nil {
				mpdRet.err = err
				sd.Logger().Error("MakeProofData: failed to encode proof data", "proof data", proofData, "err", err)
				return
			}

			if sd.estimateTxOverSize(proofDataBytes) {
				break
			}

			lastProofDataBytes = proofDataBytes
			lastTx3Hashes = tx3Hashes
			lastTx3Count = uint64(len(tx3Hashes))
			if lastTx3Count < tx3Count {
				ended = true
				break
			}

			tx3Count += step //try to contain 10 more tx3
		}

		mpdRet.proofDataBytes = lastProofDataBytes
		mpdRet.tx3Hashes = lastTx3Hashes
		mpdRet.tx3Count = lastTx3Count

		if ended {
			mpdRet.ended = true
		}
	}
	sd.Logger().Infof("MakeProofData proof data length: %d， tx3BeginIndex is %v, tx3Count is %v",
						len(proofDataBytes), tx3BeginIndex, mpdRet.tx3Count)

	return
}

func (sd *SendData) estimateTxOverSize(input []byte) bool {

	tx := ethTypes.NewTransaction(uint64(0), pabi.ChainContractMagicAddr, nil, 0, common.Big256, input)

	signedTx, _ := ethTypes.SignTx(tx, sendTxVars.signer, sendTxVars.prv)

	return signedTx.Size() >= core.TxMaxSize
}

// SendDataToMainChain send epoch data to main chain through eth_sendRawTransaction
func (sd *SendData) SendDataToMainChain(data []byte, nonce *uint64) (common.Hash, error) {

	var hash = common.Hash{}

	// data
	bs, err := pabi.ChainABI.Pack(pabi.SaveDataToMainChain.String(), data)
	if err != nil {
		log.Errorf("SendDataToMainChain, pack err: %v", err)
		return hash, err
	}

	apiBridge := sd.CCH().GetApiBridgeFromMainChain()

	// gasPrice
	gasPrice, err := apiBridge.GasPrice(sendTxVars.ctx)
	if err != nil {
		log.Errorf("SendDataToMainChain, WrpSuggestGasPrice err: %v", err)
		return hash, err
	}

	// nonce, fetch the nonce first, if we get nonce too low error, we will manually add the value until the error gone
	if *nonce == 0 {
		hexNonce, err := apiBridge.GetTransactionCount(sendTxVars.ctx, sendTxVars.account, rpc.PendingBlockNumber)
		if err != nil {
			log.Errorf("SendDataToMainChain, WrpNonceAt err: %v", err)
			return hash, err
		}
		*nonce = uint64(*hexNonce)
	}

	// tx
	tx := ethTypes.NewTransaction(*nonce, pabi.ChainContractMagicAddr, nil, 0, gasPrice.ToInt(), bs)

	// sign the tx
	signedTx, err := ethTypes.SignTx(tx, sendTxVars.signer, sendTxVars.prv)
	if err != nil {
		log.Errorf("SendDataToMainChain, SignTx err: %v", err)
		return hash, err
	}

	// eth_sendRawTransaction
	txData, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return hash, err
	}

	hash, err = apiBridge.SendRawTransaction(sendTxVars.ctx, txData)
	if err != nil {
		log.Errorf("SendDataToMainChain, WrpSendTransaction err: %v", err)
		return hash, err
	}

	if err != nil {
		log.Errorf("SendDataToMainChain, failed with: %v", err)
	} else {
		log.Infof("SendDataToMainChain, succeeded with hash: %x", hash)
	}

	return hash, err
}


func newTX3ProofData(block *ethTypes.Block) (*ethTypes.TX3ProofData, error) {
	ret := &ethTypes.TX3ProofData{
		Header: block.Header(),
	}

	txs := block.Transactions()
	// build the Trie (see derive_sha.go)
	keybuf := new(bytes.Buffer)
	trie := new(trie.Trie)
	for i := 0; i < txs.Len(); i++ {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		trie.Update(keybuf.Bytes(), txs.GetRlp(i))
	}
	// do the Merkle Proof for the specific tx
	for i, tx := range txs {
		if pabi.IsPChainContractAddr(tx.To()) {
			data := tx.Data()
			function, err := pabi.FunctionTypeFromId(data[:4])
			if err != nil {
				continue
			}

			if function == pabi.WithdrawFromChildChain {
				kvSet := ethTypes.MakeBSKeyValueSet()
				keybuf.Reset()
				rlp.Encode(keybuf, uint(i))
				if err := trie.Prove(keybuf.Bytes(), 0, kvSet); err != nil {
					return nil, err
				}

				ret.TxIndexs = append(ret.TxIndexs, uint(i))
				ret.TxProofs = append(ret.TxProofs, kvSet)
			}
		}
	}

	return ret, nil
}

func NewChildChainProofDataV1(block *ethTypes.Block, tx3BeginIndex, tx3Count uint64) (*ethTypes.ChildChainProofDataV1, []common.Hash, error) {

	txs := block.Transactions()
	if tx3BeginIndex > uint64(len(txs)) {
		return nil, nil, ExceedTxCount
	}

	// build the Trie (see derive_sha.go)
	keybuf := new(bytes.Buffer)
	trie := new(trie.Trie)
	for i := 0; i < txs.Len(); i++ {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		trie.Update(keybuf.Bytes(), txs.GetRlp(i))
	}

	proofData := &ethTypes.ChildChainProofDataV1{
		Header: block.Header(),
	}

	tx3hashes := make([]common.Hash, 0)
	// do the Merkle Proof for the specific tx
	for i, tx := range txs {

		if uint64(len(tx3hashes)) >= tx3Count {
			break
		}

		if pabi.IsPChainContractAddr(tx.To()) {
			data := tx.Data()
			function, err := pabi.FunctionTypeFromId(data[:4])
			if err != nil {
				continue
			}

			if function == pabi.WithdrawFromChildChain {
				kvSet := ethTypes.MakeBSKeyValueSet()
				keybuf.Reset()
				rlp.Encode(keybuf, uint(i))
				if err := trie.Prove(keybuf.Bytes(), 0, kvSet); err != nil {
					return nil, nil, err
				}

				if uint64(i) >= tx3BeginIndex {
					proofData.TxIndexs = append(proofData.TxIndexs, uint(i))
					proofData.TxProofs = append(proofData.TxProofs, kvSet)

					tx3hashes = append(tx3hashes, tx.Hash())
				}
			}
		}
	}

	return proofData, tx3hashes, nil
}
