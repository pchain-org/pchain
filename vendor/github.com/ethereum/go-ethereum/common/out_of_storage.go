package common

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
)

var RewardPrefix = []byte("w") // rewardPrefix + address + num (uint64 big endian) -> reward value
var RewardExtractPrefix = []byte("extrRwd-epoch-")
var OosLastBlockKey = []byte("oos-last-block")
var ProposedInEpochPrefix = []byte("proposed-in-epoch-")
var StartMarkProposalInEpochPrefix = []byte("sp-in-epoch-")

const Uint64Len  = 8

func EncodeUint64(number uint64) []byte {
	enc := make([]byte, Uint64Len)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func DecodeUint64(raw []byte) uint64 {
	return binary.BigEndian.Uint64(raw)
}


func RewardKey(address Address, epochNo uint64) []byte {
	return append(append(RewardPrefix, address.Bytes()...), EncodeUint64(epochNo)...)
}


const INV_HEIGHT = 0xffffffffffffffff
const INV_EPOCH  = 0xffffffffffffffff
const NONE_REWARD = 0

const OOS_CACHE_SIZE = 5	//OneBlockReward size - the number of cached epoch reward
const DFLT_START = 10
type OneBlockReward struct {
	Height uint64
	Reward *big.Int
}

type OBRArray struct {
	ObrArray [OOS_CACHE_SIZE]OneBlockReward
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

type OneExtracReward struct {
	Height  uint64
	Epoch   uint64
}

type XTRArray struct {
	XtrArray [OOS_CACHE_SIZE]OneExtracReward
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