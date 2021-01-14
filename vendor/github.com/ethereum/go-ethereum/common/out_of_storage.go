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

func EncodeUint64(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func DecodeUint64(raw []byte) uint64 {
	return binary.BigEndian.Uint64(raw)
}


func RewardKey(address Address, epochNo uint64) []byte {
	return append(append(RewardPrefix, address.Bytes()...), EncodeUint64(epochNo)...)
}


const OBR_SIZE = 5
const INV_HEIGHT = 0xffffffffffffffff
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
	for i:=0; i<OBR_SIZE; i++ {
		obrArray.ObrArray[i].Height = INV_HEIGHT
		obrArray.ObrArray[i].Reward = big.NewInt(0)
	}

	if err := rlp.DecodeBytes(byteArray, &obrArray); err != nil {
		return obrArray, err
	}
	return obrArray, nil
}