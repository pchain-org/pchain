package rawdb

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	//out of storage
	RewardPrefix                   = []byte("w") // rewardPrefix + address + num (uint64 big endian) -> reward value
	RewardExtractPrefix            = []byte("extrRwd-epoch-") // rewardExtractPrefix + address -> rewardExtract value
	OosLastBlockKey                = []byte("oos-last-block")
	ProposedInEpochPrefix          = []byte("proposed-in-epoch-") // proposedInEpochPrefix + address + num (uint64 big endian) -> proposedInEpoch value
	StartMarkProposalInEpochPrefix = []byte("sp-in-epoch-")
)

const INV_HEIGHT = 0xffffffffffffffff
const INV_EPOCH  = 0xffffffffffffffff
const NONE_REWARD = 0
const Uint64Len  = 8

const OBR_SIZE = 5	//OneBlockReward size - the number of cached epoch reward
const DFLT_START = 10
type OneBlockReward struct {
	Height uint64
	Reward *big.Int
}

type OBRArray struct {
	ObrArray [OBR_SIZE]OneBlockReward
}

func OBRArray2Bytes(obrArray OBRArray) ([]byte, error) {
	return rlp.EncodeToBytes(obrArray)
}

func Bytes2OBRArray(byteArray []byte) (OBRArray, error) {
	obrArray := OBRArray{}
	if err := rlp.DecodeBytes(byteArray, &obrArray); err != nil {
		return obrArray, err
	}
	return obrArray, nil
}

const XTR_SIZE = 5 //XTR_SIZE size - the number of cached extrRwd operations
type OneExtracReward struct {
	Height  uint64
	Epoch   uint64
}

type XTRArray struct {
	XtrArray [XTR_SIZE]OneExtracReward
}

func XTRArray2Bytes(xtrArray XTRArray) ([]byte, error) {
	return rlp.EncodeToBytes(xtrArray)
}

func Bytes2XTRArray(byteArray []byte) (XTRArray, error) {
	xtrArray := XTRArray{}
	if err := rlp.DecodeBytes(byteArray, &xtrArray); err != nil {
		return xtrArray, err
	}
	return xtrArray, nil
}


func EncodeUint64(number uint64) []byte {
	enc := make([]byte, Uint64Len)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func DecodeUint64(raw []byte) uint64 {
	return binary.BigEndian.Uint64(raw)
}

func RewardKey(address []byte, epochNo uint64) []byte {
	return append(append(RewardPrefix, address...), EncodeUint64(epochNo)...)
}

func ReadReward(db ethdb.Database, address common.Address, epoch uint64) *big.Int {
	reward, _ := db.Get(RewardKey(address.Bytes(), epoch))
	if len(reward) == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(reward)
}

//fill {height, reward} to current obrArray - OneBlockReward Array - with following rules:
//1 if there is empty(invalid)/equal/bigger item, fill it with {height, reward}, and mark other items with bigger height to invalid
//2 if there no empty(invalid) item, find one with smallest height, fill it with {height, reward}
//the OBR_SIZE is 5 now, so the implemetation is just iterate all items, if it is bigger, could consider sorting
func WriteReward(db ethdb.Database, address common.Address, epoch uint64, height uint64, reward *big.Int) {
	/*
		if err := db.Put(rewardKey(address, epoch), reward.Bytes()); err != nil {
			log.Crit("Failed to store epoch reward", "err", err)
		}
	*/
	oriReward, _ := db.Get(RewardKey(address.Bytes(), epoch))
	obr := OBRArray{}
	initIndex := 0


	err := errors.New("")
	if len(oriReward) != 0 {
		obr, err = Bytes2OBRArray(oriReward)
		if err != nil {
			obr.ObrArray[0].Height = DFLT_START
			obr.ObrArray[0].Reward = new(big.Int).SetBytes(oriReward)
			initIndex = 1
		}else {
			initIndex = OBR_SIZE
		}
	}
	for i:=initIndex; i< OBR_SIZE; i++ {
		obr.ObrArray[i].Height = INV_HEIGHT
		obr.ObrArray[i].Reward = big.NewInt(NONE_REWARD)
	}

	minIndex := 0
	minHeight := uint64(INV_HEIGHT)
	settled := false
	for i:=0; i< OBR_SIZE; i++ {
		key := obr.ObrArray[i].Height
		if key >= height {
			if !settled {
				obr.ObrArray[i].Height = height
				obr.ObrArray[i].Reward = reward
				settled = true
			} else if key != INV_HEIGHT {
				obr.ObrArray[i].Height = INV_HEIGHT
				obr.ObrArray[i].Reward = big.NewInt(NONE_REWARD)

			}
		} else {
			if minHeight == INV_HEIGHT || key < minHeight {
				minIndex = i
				minHeight = key
			}
		}
	}

	if !settled {
		obr.ObrArray[minIndex].Height = height
		obr.ObrArray[minIndex].Reward = reward
	}

	rewardBytes, err := OBRArray2Bytes(obr)
	if err != nil {
		log.Crit("Failed to convert epoch reward", "err", err)
	}

	if err := db.Put(RewardKey(address.Bytes(), epoch), rewardBytes); err != nil {
		log.Crit("Failed to store epoch reward", "err", err)
	}
}

//
//func DeleteReward(db ethdb.Writer, address common.Address, epoch uint64) {
//	if err := db.Delete(rewardKey(address, epoch)); err != nil {
//		log.Crit("Failed to delete epoch reward", "err", err)
//	}
//}

func GetEpochReward(db ethdb.Database, address common.Address, epoch uint64, height uint64) *big.Int {
	reward, _ := db.Get(append(append(RewardPrefix, address.Bytes()...), EncodeUint64(epoch)...))
	if len(reward) == 0 {
		return big.NewInt(0)
	} else {
		obrArray, err := Bytes2OBRArray(reward)
		if err == nil {
			closestIndex := 0
			closestHeight := uint64(INV_HEIGHT)
			for i:=0; i< OBR_SIZE; i++ {
				key := obrArray.ObrArray[i].Height
				if key == height {
					return obrArray.ObrArray[i].Reward
				} else if key < height {
					if closestHeight == INV_HEIGHT || key > closestHeight {
						closestIndex = i
						closestHeight = key
					}
				}
			}

			if closestHeight != INV_HEIGHT {
				return obrArray.ObrArray[closestIndex].Reward
			}

			return big.NewInt(0)
		}

		return new(big.Int).SetBytes(reward)
	}
}

func GetAllEpochReward(db ethdb.Database, address common.Address, height uint64) map[uint64]*big.Int {
	it := db.NewIteratorWithPrefix(append(RewardPrefix, address.Bytes()...))
	defer it.Release()

	result := make(map[uint64]*big.Int)
	for it.Next() {
		epoch := DecodeUint64(it.Key()[21:])
		reward := GetEpochReward(db, address, epoch, height)
		result[epoch] = reward
	}
	return result
}

func WriteEpochRewardExtracted(db ethdb.Database, address common.Address, epoch uint64, height uint64) error {

	oriEpoch, _ := db.Get(append(RewardExtractPrefix, address.Bytes()...))

	xtr := XTRArray{}
	initIndex := 0

	err := errors.New("")
	if len(oriEpoch) != 0 {
		xtr, err = Bytes2XTRArray(oriEpoch)
		if err != nil {
			xtr.XtrArray[0].Height = DFLT_START
			xtr.XtrArray[0].Epoch = DecodeUint64(oriEpoch)
			initIndex = 1
		} else {
			initIndex = XTR_SIZE
		}
	}

	for i:=initIndex; i< XTR_SIZE; i++ {
		xtr.XtrArray[i].Height = INV_HEIGHT
		xtr.XtrArray[i].Epoch = INV_EPOCH
	}

	minIndex := 0
	minHeight := uint64(INV_HEIGHT)
	settled := false
	for i:=0; i< XTR_SIZE; i++ {
		key := xtr.XtrArray[i].Height
		if key >= height {
			if !settled {
				xtr.XtrArray[i].Height = height
				xtr.XtrArray[i].Epoch = epoch
				settled = true
			} else if key != INV_HEIGHT {
				xtr.XtrArray[i].Height = INV_HEIGHT
				xtr.XtrArray[i].Epoch = INV_EPOCH
			}
		} else {
			if minHeight == INV_HEIGHT || key < minHeight {
				minIndex = i
				minHeight = key
			}
		}
	}

	if !settled {
		xtr.XtrArray[minIndex].Height = height
		xtr.XtrArray[minIndex].Epoch = epoch
	}

	epochBytes, err := XTRArray2Bytes(xtr)
	if err != nil {
		log.Crit("Failed to convert extract_reward", "err", err)
		return err
	}

	if err := db.Put(append(RewardExtractPrefix, address.Bytes()...), epochBytes); err != nil {
		log.Crit("Failed to store extract_reward", "err", err)
		return err
	}

	return nil
}

func GetEpochRewardExtracted(db ethdb.Database, address common.Address, height uint64) (uint64, error) {

	epochBytes, err := db.Get(append(RewardExtractPrefix, address.Bytes()...))

	if err != nil {
		return INV_EPOCH, err
	}

	if len(epochBytes) == 0 {
		log.Errorf("data error, no epoch for reward_extract readed")
		return INV_EPOCH, nil
	} else {
		xtrArray, err := Bytes2XTRArray(epochBytes)
		if err == nil {
			closestIndex := 0
			closestHeight := uint64(INV_HEIGHT)
			hasInvalidKey := false
			for i:=0; i< XTR_SIZE; i++ {
				key := xtrArray.XtrArray[i].Height
				if key == height {
					return xtrArray.XtrArray[i].Epoch, nil
				} else if key < height {
					if closestHeight == INV_HEIGHT || key > closestHeight {
						closestIndex = i
						closestHeight = key
					}
				} else if key == INV_HEIGHT {
					hasInvalidKey = true
				}
			}

			if closestHeight != INV_HEIGHT {
				return xtrArray.XtrArray[closestIndex].Epoch, nil
			}

			if !hasInvalidKey {
				log.Crit("data error, no epoch for reward_extract readed; " +
					"if it is during rollback, need to catchup from 0 block or restore from a snapshot")
				return INV_EPOCH, nil
			} else {
				return INV_EPOCH, errors.New("no Extract Mark yet")
			}

		}

		return DecodeUint64(epochBytes), nil
	}
}

func ReadOOSLastBlock(db ethdb.Database) (*big.Int, error) {
	blockBytes, err := db.Get(OosLastBlockKey)

	if err != nil {
		return big.NewInt(-1), err
	}

	return new(big.Int).SetBytes(blockBytes), nil
}

func WriteOOSLastBlock(db ethdb.Database, blockNumber *big.Int) error {
	return db.Put(OosLastBlockKey, blockNumber.Bytes())
}

func MarkProposedInEpoch(db ethdb.Database, address common.Address, epoch, height uint64) error {

	epochBytes, err := db.Get(append(append(ProposedInEpochPrefix, address.Bytes()...), EncodeUint64(epoch)...))
	countValue := uint64(0)
	firstProposedBlock := uint64(0)
	if err == nil && len(epochBytes) % 8 == 0 {
		countBytes := make([]byte, 8)
		copy(countBytes, epochBytes[0:8])
		countValue = DecodeUint64(countBytes)
		if len(epochBytes) == 16 {
			firstProposedBlockBytes := make([]byte, 8)
			copy(firstProposedBlockBytes, epochBytes[8:])
			firstProposedBlock = DecodeUint64(firstProposedBlockBytes)
		}
	}

	countValue ++
	if firstProposedBlock == 0 || firstProposedBlock > height {
		firstProposedBlock = height
	}

	return db.Put(append(
		append(ProposedInEpochPrefix, address.Bytes()...), EncodeUint64(epoch)...),
		append(EncodeUint64(countValue), EncodeUint64(firstProposedBlock)...))
}

func CheckProposedInEpoch(db ethdb.Database, address common.Address, epoch uint64) (uint64, bool) {
	epochBytes, err := db.Get(append(append(ProposedInEpochPrefix, address.Bytes()...), EncodeUint64(epoch)...))
	if err != nil {
		return 0, false
	}

	length := len(epochBytes)
	if length == 8 {
		countValue := DecodeUint64(epochBytes)
		if countValue != 1 {
			log.Error("value for count of proposed block must be 1, now is %v", countValue)
			return 0, false
		}
		return 0, true
	} else if length == 16 {
		countBytes := make([]byte, 8)
		copy(countBytes, epochBytes[0:8])
		firstProposedBlockBytes := make([]byte, 8)
		copy(firstProposedBlockBytes, epochBytes[8:])
		countValue := DecodeUint64(countBytes)
		firstProposedBlock := DecodeUint64(firstProposedBlockBytes)
		if countValue <= 0 {
			log.Error("value for count of proposed block must be larger then 0, now is %v", countValue)
			return 0, false
		}
		return firstProposedBlock, true
	}
	log.Error("value for count of proposed block does not get, return true to continue")
	return 0, true
}

func ClearProposedInEpoch(db ethdb.Database, address common.Address, epoch uint64) error {
	return db.Delete(append(append(ProposedInEpochPrefix, address.Bytes()...), EncodeUint64(epoch)...))
}

func MarkProposalStartInEpoch(db ethdb.Database, epoch uint64) error {
	return db.Put(StartMarkProposalInEpochPrefix, EncodeUint64(epoch))
}

func GetProposalStartInEpoch(db ethdb.Database) (uint64, error) {

	epochBytes, err := db.Get(StartMarkProposalInEpochPrefix)

	if err != nil {
		return 0xffffffffffffffff, err
	}

	return DecodeUint64(epochBytes), nil
}

