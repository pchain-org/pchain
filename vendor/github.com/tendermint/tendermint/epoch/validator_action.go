package epoch

import (
	"fmt"
	"sync"
	"os"
	"bytes"
	"errors"
	dbm "github.com/tendermint/go-db"
	wire "github.com/tendermint/go-wire"
	"strings"
	ethCoreTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/tendermint/types"
)

/*
VO_{epoch}: val1 : val2: val3
VO_{epoch}_{validator}: txhash1, txhash2, txhash3
VO_{txhash}: index, txhash, action, amount, epoch, voteset, confirmed, confirmedblock
*/

/*
table1: candidate_validator
0x7e12...
[
{index:0,txhash:ab, action:join, amount:10, epoch:100, voteset{0xa, 0xb, .., 0xe}, confirmed:false, confirmedblock:-1},
{index:1,txhash:ac, action:join, amount:10, epoch:100, voteset{0xa, 0xb, .., 0xe}, confirmed:false, confirmedblock:-1},
]

table2: validator_epoch
100[
0x7e12...
[
{txhash:ab, index:0, action:join, amount:10},
{txhash:ac, index:1, action:withdraw, amount:5}
]
]
 */

const (
	SVM_JOIN = "join"
	SVM_WITHDRAW = "withdraw"
	SVM_ACCEPT = "accept"
)

var VA_UNCONFIRMED_EPOCH = -1
var VA_SEPARATOR = ";"
var VO_Key = "ValidatorOperation"

var VADB dbm.DB

var NextValidatorOperationNotExist = errors.New("the operator for validator does not support")

const (
	VALIDATOR_ACTION_NOT_EXIST = iota	     		// value --> 0
)

type ValidatorOperation struct {

	Validator string
	PubKey []byte
	Index int
	TxHash string
	Action string
	Amount uint64
	Epoch  int
	VoteSet []string //the set for voting validators; not record the voters' power
	Confirmed bool
	ConfirmedBlock int
}

//candidate operation mapped by validators
type ValidatorOperationSet struct {
	mtx sync.Mutex

	Epoch int
	Operations map[string][]*ValidatorOperation
}


func calcEpochKey(epoch int) []byte {
	if epoch < 0 {
		epoch = VA_UNCONFIRMED_EPOCH
	}

	return []byte(VO_Key + fmt.Sprintf("_%s", epoch))
}

func calcValidatorKey(validator string, epoch int) []byte {
	if epoch < 0 {
		epoch = VA_UNCONFIRMED_EPOCH
	}

	return []byte(VO_Key + fmt.Sprintf("_%s", epoch) + validator)
}

func calcHashKey(txhash string) []byte {
	return []byte(VO_Key + txhash)
}

//when epoch < 0, will return unconfirmed validator operations, or return confirmed operations in specific epoch
func LoadValidatorOperationSet(epoch int) *ValidatorOperationSet {

	if epoch < 0 {
		epoch = VA_UNCONFIRMED_EPOCH
	}

	buf := VADB.Get(calcEpochKey(epoch))
	if len(buf) == 0 {
		return nil
	} else {

		cv := &ValidatorOperationSet{
			Epoch: epoch,
			Operations : make(map[string][]*ValidatorOperation),
		}

		cvStrArray := strings.Split(string(buf), VA_SEPARATOR)
		cvLen := len(cvStrArray)
		if cvLen == 0 {
			return nil
		}
		for i:=0; i<cvLen; i++ {
			validator := cvStrArray[i]
			co := loadOperationWithValidator(validator, epoch)
			if co != nil {
				cv.Operations[validator] = co
			}
		}

		return cv
	}
}

func loadOperationWithValidator(validator string, epoch int) []*ValidatorOperation{
	if epoch < 0 {
		epoch = VA_UNCONFIRMED_EPOCH
	}

	buf := VADB.Get(calcValidatorKey(validator, epoch))
	if len(buf) == 0 {
		return nil
	} else {
		result := []*ValidatorOperation{}

		hashStrArray := strings.Split(string(buf), VA_SEPARATOR)
		hashLen := len(hashStrArray)
		if hashLen == 0 {
			return nil
		}
		for i:=0; i<hashLen; i++ {
			txhash := hashStrArray[i]
			vo := LoadOperationWithHash(txhash)
			if vo != nil {
				result = append(result, vo)
			}
		}

		return result
	}
}

func LoadOperationWithHash(txhash string) *ValidatorOperation{

	vo := &ValidatorOperation{}
	buf := VADB.Get(calcHashKey(txhash))
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int), new(error)
		wire.ReadBinaryPtr(&vo, r, 0, n, err)
		if *err != nil {
			// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
			fmt.Printf("LoadCandidateOperation: Data has been corrupted or its spec has changed: %v\n", *err)
			os.Exit(1)
		}
		fmt.Printf("LoadCandidateHash(), ValidatorOperation is: %v\n", vo)
		return vo
	}
}

func AddValidatorOperation(voSet *ValidatorOperationSet, epoch int, vo *ValidatorOperation) (*ValidatorOperationSet, error) {

	if vo == nil {
		return nil, nil
	}

	if voSet == nil {

		voSet = LoadValidatorOperationSet(epoch)

		if voSet == nil {
			voSet = &ValidatorOperationSet{
				Epoch: epoch,
				Operations: make(map[string][]*ValidatorOperation),
			}
		}

		vos := voSet.Operations[vo.Validator]
		var found bool = false
		for i:=0; i<len(vos); i++ {
			if vos[i].TxHash == vo.TxHash {
				fmt.Printf("validator not added because there exists one\n")
				found = true
				break
			}
		}
		if !found {
			vo.Index = len(vos)
			vos = append(vos, vo)
		}

		voSet.Operations[vo.Validator] = vos
	}

	return voSet, nil
}

func RemoveValidatorOperation(voSet *ValidatorOperationSet, vo *ValidatorOperation) error {

	if voSet == nil {
		return nil
	}

	vos := voSet.Operations[vo.Validator]
	newVos := make([]*ValidatorOperation, 0)
	for i:=0; i<len(vos); i++ {
		if(vos[i].TxHash != vo.TxHash) {
			newVos = append(newVos, vos[i])
		}
	}
	voSet.Operations[vo.Validator] = newVos

	return nil
}

func (co *ValidatorOperationSet)Save() error {

	co.mtx.Lock()
	defer co.mtx.Unlock()

	var cvStr string = ""
	var first bool = true
	for validator, operations := range co.Operations {
		if first {
			cvStr += validator
		} else {
			cvStr += VA_SEPARATOR + validator
		}

		err := co.saveOperationWithValidator(validator, operations)
		if err != nil {
			return err
		}

		first = false
	}

	if cvStr != "" {
		VADB.Set(calcEpochKey(co.Epoch), []byte(cvStr))
	}

	return nil
}

func (co *ValidatorOperationSet)saveOperationWithValidator(validator string, operations []*ValidatorOperation) error {

	var hashStr string = ""
	for i:=0; i<len(operations); i++ {

		operation := operations[i]

		hash := operation.TxHash

		if i == 0 {
			hashStr += hash
		} else {
			hashStr += VA_SEPARATOR + validator
		}

		err := co.saveOperationWithHash(hash, operation)
		if err != nil {
			return err
		}
	}

	if hashStr != "" {
		VADB.Set(calcValidatorKey(validator, co.Epoch), []byte(hashStr))
	}

	return nil
}

func (co *ValidatorOperationSet)saveOperationWithHash(hash string, operation *ValidatorOperation) error {

	buf, n, err := new(bytes.Buffer), new(int), new(error)
	fmt.Printf("saveOperationWithHash(), hash is %s\n", hash)

	wire.WriteBinary(operation, buf, n, err)
	if *err != nil {
		fmt.Printf("Epoch get bytes error: %v", err)
	}

	VADB.SetSync(calcHashKey(hash), buf.Bytes())
	fmt.Printf("saveOperationWithHash(), (buf, n) are: (%X, %v)\n", buf.Bytes(), *n)

	return nil
}


var UnlockAssertFuncName = "UnlockAssert"

func NewUnlockAssetTransaction(sender string, account string, amount uint64) (types.Tx, error){

	params := ethCoreTypes.MakeKeyValueSet()
	params.Set("sender", sender)
	params.Set("account", account)
	params.Set("amount", amount)

	etd := &ethCoreTypes.ExtendTxData {
		FuncName:    UnlockAssertFuncName,
		Params:      params,
	}

	return types.NewEthTransaction(sender, etd)
}
