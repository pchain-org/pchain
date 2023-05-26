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


func (sc *ConsensusState) CurrentCCTBlock() *big.Int {
	return sc.sendData.CurrentCCTBlock()
}

func (sc *ConsensusState) WriteCurrentCCTBlock(blockNumber *big.Int) {
	sc.sendData.WriteCurrentCCTBlock(blockNumber)
}

func (sc *ConsensusState) GetLatestCCTExecStatus(hash common.Hash) *ethTypes.CCTTxExecStatus {
	return sc.sendData.GetLatestCCTExecStatus(hash)
}

func (sc *ConsensusState) GetCCTExecStatusByHash(hash common.Hash) []*ethTypes.CCTTxExecStatus {
	return sc.sendData.GetCCTExecStatusByHash(hash)
}

func (sc *ConsensusState) WriteCCTExecStatus(receipt *ethTypes.CCTTxExecStatus) {
	sc.sendData.WriteCCTExecStatus(receipt)
}

func (sc *ConsensusState) DeleteCCTExecStatus(hash common.Hash) {
	sc.sendData.DeleteCCTExecStatus(hash)
}

func (sc *ConsensusState) SignTx(tx *ethTypes.Transaction) (*ethTypes.Transaction, error) {
	//initialize sendTxVars
	prv, err := crypto.ToECDSA(sc.privValidator.(*types.PrivValidator).PrivKey.(tmdcrypto.BLSPrivKey).Bytes())
	if err != nil {
		return nil, err
	}

	digest := crypto.Keccak256([]byte(sc.chainConfig.PChainId))
	signer := ethTypes.LatestSignerForChainID(new(big.Int).SetBytes(digest[:]))

	return ethTypes.SignTx(tx, signer, prv)
}

var (
	indexKeyFmt     = "Data-%x" //%x is blockNumber
	indexKeyPrefix  = []byte("Data-")
	indexValue      = []byte("~o~") //position occupation
	contentKeyFmt   = "Content-%x"  //%x is the number of which block to send
	contentKeyPrefix = []byte("Content-")

	cctBlockKey     = []byte("CCTBlock-")

	receiptKeyFmt      = "CCTReceipt-%x"

	ExceedTxCount   = errors.New("exceed the tx count")
)


type SDItem struct{

	//only parent item would be stored in db
	isParent        bool
	//for parent, key is block number string, for child, key is TxBeginindex
	children        map[string]*SDItem
	parent          *SDItem
	proofDataBytes  []byte
	txHashes        []common.Hash

	BlockNumber     uint64
	EpochNumber     int //less than zero means no epoch information
	//when the count of txs is too many, split them into different sd2vc txs
	//for example there is 230 txes, send them with different 3 sd2vc txes which may contains tx
	//like (TxBeginIndex, TxCount) = (0, 100), (100, 100), (200, 30]
	TxBeginIndex    uint64
	TxCount         uint64 //equcal zero means no tx
	SentTimes       int
	Hash            common.Hash //if sd2mc tx has sent, here is the hash

}

type SendData struct {
	DB              ethdb.Database
	dbMtx           sync.Mutex

	items           map[string]*SDItem //string should be "blockNumber" or "blockNumber+txBeginIndex+txCount"

	cctBlock        *big.Int

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
		parentItem.children[calcMapKey(item.TxBeginIndex)] = item
	}

	return nil
}

//only parent item needs db operation
func (sd *SendData) deleteDataItem (isParent bool, blockNumber, beginIndex uint64) error {

	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	sdItem := sd.items[calcMapKey(blockNumber)]
	deleteParent := isParent
	if !deleteParent {
		delete(sdItem.children, calcMapKey(beginIndex))
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
			sd.Logger().Error("checkOneBlock: tx not found in main chain, send again", "hash", hash)
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
					if picked.TxCount > 0 {
						sd.checkTxInMainChain(block)
					}
				}
				sd.deleteDataItem(picked.isParent, picked.BlockNumber, picked.TxBeginIndex)
			}
		}
	}
}

func (sd *SendData) checkEpochInMainChain(block *ethTypes.Block) error {
	sd.Logger().Infof("CheckEpochInMainChain: block %v epoch was stored in main chain", block.NumberU64())
	return nil
}

func (sd *SendData) checkTxInMainChain(block *ethTypes.Block) error {
	sd.Logger().Infof("checkTxInMainChain: block %v tx was stored in main chain", block.NumberU64())
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
			picked.BlockNumber, picked.TxBeginIndex, picked.TxCount)
	} else {
		sd.Logger().Debug("pickOneForSend: no block is picked to send")
	}

	return picked
}

type MPDRet struct {
	proofDataBytes []byte
	epochNumber    int
	txHashes       []common.Hash
	txCount        uint64
	iterCount      uint64
	ended          bool
	err            error
}

func (sd *SendData) sendOneItem(picked *SDItem, version int) error {
	sd.runMtx.Lock()
	defer sd.runMtx.Unlock()

	proofDataBytes := picked.proofDataBytes
	beginIndex := uint64(0)

	if len(proofDataBytes) == 0 {
		block := sd.ChainReader().GetBlockByNumber(picked.BlockNumber)

		ended := false
		isParent := true
		for !ended {
			ret := sd.MakeProofData(block, beginIndex, version)
			if ret.err != nil {
				if ret.err == ExceedTxCount {
					break
				} else {
					return ret.err
				}
			}

			if !ret.ended && isParent {
				isParent = false
			}
			if len(ret.proofDataBytes) != 0 {
				item := &SDItem{
					isParent: isParent,
					BlockNumber: picked.BlockNumber,
					EpochNumber: ret.epochNumber,
					txHashes: ret.txHashes,
					TxBeginIndex: beginIndex,
					TxCount: ret.txCount,
					proofDataBytes: ret.proofDataBytes,
				}
				sd.addOrUpdateDataItem(item)
			}

			beginIndex += ret.iterCount
			ended = ret.ended
		}

		picked = sd.items[calcMapKey(picked.BlockNumber)]
		if len(picked.children) != 0 {
			picked = sd.items[calcMapKey(picked.BlockNumber)].children[calcMapKey(uint64(0))]
		}

		proofDataBytes = picked.proofDataBytes
	}

	hash, err := sd.SendDataToMainChain(proofDataBytes, &sendTxVars.nonceCache)
	if err == nil {
		picked.SentTimes++
		picked.Hash = hash
		sd.addOrUpdateDataItem(picked)
		if picked.TxCount > 0 {
			sd.Logger().Infof("sendDataToMainChain with (%v-%v-%v), first hashe is:",
				picked.BlockNumber, picked.TxBeginIndex, picked.TxCount)
			sd.Logger().Infof("%x ", picked.txHashes[0])
			//for _,hash := range picked.txHashes {
			//	sd.Logger().Infof("%x ", hash)
			//}
		}

		sendTxVars.nonceCache ++
	} else {
		sendTxVars.nonceCache = 0
	}
	
	return err
}

func (sd *SendData) MakeProofData(block *ethTypes.Block, beginIndex uint64, version int) (mpdRet *MPDRet) {

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
		lastTxHashes := make([]common.Hash,0)
		lastTxCount := uint64(0)
		lastIterCount := uint64(0)
		lastProofDataBytes := []byte{}
		step := uint64(10)
		count := step
		for ;; {
			proofData, hashes, iCount, err := NewChildChainProofDataV1(block, beginIndex, count)
			if err == ExceedTxCount {
				mpdRet.err = err
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
			lastTxHashes = hashes
			lastTxCount = uint64(len(proofData.TxProofs))
			lastIterCount = iCount

			if iCount < count {
				mpdRet.ended = true
				break
			}

			count += step //try to contain 10 more tx
		}

		mpdRet.proofDataBytes = lastProofDataBytes
		mpdRet.txHashes = lastTxHashes
		mpdRet.txCount = lastTxCount
		mpdRet.iterCount = lastIterCount
	}
	sd.Logger().Infof("MakeProofData proof data length: %d， beginIndex is %v, iterCount is %v, txCount is %v",
						len(proofDataBytes), beginIndex, mpdRet.iterCount, mpdRet.txCount)

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


func (sd *SendData) CurrentCCTBlock() *big.Int {
	if sd.cctBlock != nil {
		return new(big.Int).Set(sd.cctBlock)
	}

	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	bnBytes, err := sd.DB.Get(cctBlockKey)
	if err != nil {
		return nil
	}

	return new(big.Int).SetBytes(bnBytes)
}

func (sd *SendData) WriteCurrentCCTBlock(blockNumber *big.Int) {
	if sd.cctBlock == nil || blockNumber.Cmp(sd.cctBlock) > 0 {
		sd.cctBlock = blockNumber

		sd.dbMtx.Lock()
		defer sd.dbMtx.Unlock()

		sd.DB.Put(cctBlockKey, sd.cctBlock.Bytes())
	}
}

func calReceiptKey(hash common.Hash) []byte {
	return []byte(fmt.Sprintf(receiptKeyFmt, hash))
}

func (sd *SendData) GetLatestCCTExecStatus(hash common.Hash) *ethTypes.CCTTxExecStatus {
	cctESs := sd.GetCCTExecStatusByHash(hash)
	if len(cctESs) == 0 {
		return nil
	} else {
		cctES := cctESs[0]
		for _, cts := range cctESs {
			if cts.MainBlockNumber.Cmp(cts.MainBlockNumber) > 0 {
				cctES = cts
			}
		}
		return cctES
	}
}

func (sd *SendData) GetCCTExecStatusByHash(hash common.Hash) []*ethTypes.CCTTxExecStatus {

	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	cctESBytes, err := sd.DB.Get(calReceiptKey(hash))
	if err != nil {
		return nil
	}

	cctESs := make([]*ethTypes.CCTTxExecStatus, 0)
	err = rlp.DecodeBytes(cctESBytes, &cctESs)
	if err != nil {
		return nil
	}

	return cctESs
}

func (sd *SendData) WriteCCTExecStatus(cctES *ethTypes.CCTTxExecStatus) {
	if cctES != nil {

		receipts := sd.GetCCTExecStatusByHash(cctES.TxHash)
		receipts = append(receipts, cctES)

		receiptByptes, err := rlp.EncodeToBytes(receipts)
		if err != nil {
			return
		}

		sd.dbMtx.Lock()
		defer sd.dbMtx.Unlock()

		sd.DB.Put(calReceiptKey(cctES.TxHash), receiptByptes)
	}
}

func (sd *SendData) DeleteCCTExecStatus(hash common.Hash) {

	sd.dbMtx.Lock()
	defer sd.dbMtx.Unlock()

	sd.DB.Delete(calReceiptKey(hash))
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

func NewChildChainProofDataV1(block *ethTypes.Block, beginIndex, count uint64) (*ethTypes.ChildChainProofDataV1, []common.Hash, uint64, error) {

	txs := block.Transactions()
	if beginIndex > uint64(len(txs)) {
		return nil, nil, 0, ExceedTxCount
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

	txHashes := make([]common.Hash, 0)

	// do the Merkle Proof for the specific tx
	slicedTxs := txs[beginIndex:]
	iCount := uint64(0)
	for i, tx := range slicedTxs {

		if uint64(i) >= count {
			break
		}
		iCount ++

		if pabi.IsPChainContractAddr(tx.To()) {
			data := tx.Data()
			function, err := pabi.FunctionTypeFromId(data[:4])
			if err != nil {
				continue
			}

			if function == pabi.WithdrawFromChildChain ||
				function == pabi.CrossChainTransferExec {
				kvSet := ethTypes.MakeBSKeyValueSet()
				keybuf.Reset()
				realIndex := uint(i) + uint(beginIndex)
				rlp.Encode(keybuf, realIndex)
				if err := trie.Prove(keybuf.Bytes(), 0, kvSet); err != nil {
					return nil, nil, iCount, err
				}

				proofData.TxIndexs = append(proofData.TxIndexs, realIndex)
				proofData.TxProofs = append(proofData.TxProofs, kvSet)

				txHashes = append(txHashes, tx.Hash())
			}
		}
	}

	return proofData, txHashes, iCount, nil
}
