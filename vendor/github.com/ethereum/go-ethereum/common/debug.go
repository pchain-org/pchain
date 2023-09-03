// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"fmt"
	"math/big"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
)

// Report gives off a warning requesting the user to submit an issue to the github tracker.
func Report(extra ...interface{}) {
	fmt.Fprintln(os.Stderr, "You've encountered a sought after, hard to reproduce bug. Please report this to the developers <3 https://github.com/ethereum/go-ethereum/issues")
	fmt.Fprintln(os.Stderr, extra...)

	_, file, line, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "%v:%v\n", file, line)

	debug.PrintStack()

	fmt.Fprintln(os.Stderr, "#### BUG! PLEASE REPORT ####")
}

// PrintDepricationWarning prinst the given string in a box using fmt.Println.
func PrintDepricationWarning(str string) {
	line := strings.Repeat("#", len(str)+4)
	emptyLine := strings.Repeat(" ", len(str))
	fmt.Printf(`
%s
# %s #
# %s #
# %s #
%s

`, line, emptyLine, str, emptyLine, line)
}


type BalanceReward struct {
	BeforeBalance, BeforeReward *big.Int
}

var LogAddrs = map[Address]*BalanceReward {
	HexToAddress("0x852d12801e5fb640a84421c37eafae87ba86c76c") : nil,
	HexToAddress("0xbecabc3fed76ca7a551d4c372c20318b7457878c") : nil,
	HexToAddress("0x82bc1c28bef8f31e8d61a1706dcab8d36e6f5e58") : nil, //________	(not happen)
	HexToAddress("0xceb2694a1ddb8daf849825d74c4954dcd0ad6489") : nil, //51389255	???
	HexToAddress("0xd5e6619291b2384b5b7da595a9bd78ec7ea30785") : nil,
	HexToAddress("0x39a9590fdee5f90d05360beb6cf2f4adb05a02a5") : nil,
	HexToAddress("0xeaeb9794265a4b38ddfcf69ede2f65d15fe99902") : nil, //________	(not happen)
	HexToAddress("0x9a4eb75fc8db5680497ac33fd689b536334292b0") : nil,
	HexToAddress("0xae6bde77bc386d2cb6492f824ded9147d0926512") : nil, //80356659	???
	HexToAddress("0xef470c3a63343585651808b8187bba0e277bc3c8") : nil,
	HexToAddress("0x133d604a2a138f04db8fb7d1f57fd739ad4b08aa") : nil,
	HexToAddress("0x6ea97c1d1588c589589fa0e1f66457897fa9b1cc") : nil,
	HexToAddress("0x723c1b86c78a04c4f125df4573acb0625bfc69a5") : nil,
}

func NeedLogReward(chainId string, addr Address, epoch, blockNr uint64) bool {

	if chainId == "child_0" && epoch == 19 {
		if _, exist := LogAddrs[addr]; exist {
			return true
		}
	}
	if NeedDebug(chainId, blockNr) {
		return true
	}

	return false
}


var RoughCheckSync = false
var SkipRootInconsistence = false
var bhMap = make(map[Hash]Hash)
func SetHashWithBlockNumber(old, new Hash) {
	bhMap[old] = new
}

func GetHashFromBlockNumber(old Hash) Hash {
	if hash, exist := bhMap[old]; exist {
		return hash
	}
	return old
}

//child_0
var debugBlocks = []uint64 {
	//accumulate patch
	26536499, 26536500,

	//extrRwd patch
	//tx skipped execution
	32110529, 32132151,
	//retrieve reward twice 
	//for 0xf5005b496dff7b1ba3ca06294f8f146c9afbe09d
	22094435, 
	//tx need add reward difference
	//for 0x852d12801e5fb640a84421c37eafae87ba86c76c
	33389535, 35182438, 36492381, 38070487, 41975759, 45115772, 49769059, 55788578,
	//for 0xbecabc3fed76ca7a551d4c372c20318b7457878c
	33611723, 37967696, 40807203, 42394831, 45190833, 47305502, 61201149,
	//for 0x82bc1c28bef8f31e8d61a1706dcab8d36e6f5e58
	33612352, 36624513, 40260767, 42600734,
	//for 0xceb2694a1ddb8daf849825d74c4954dcd0ad6489
	34132176, 38004652, 39482991, 41970025, 44569259, 46688263, 48756650,
	//for 0xd5e6619291b2384b5b7da595a9bd78ec7ea30785
	34517913, 38183507, 42712099, 55923898,
	//for 0x39a9590fdee5f90d05360beb6cf2f4adb05a02a5
	36553835, 38000080, 40241404, 41960852, 48221867, 57941470,
	//for 0xeaeb9794265a4b38ddfcf69ede2f65d15fe99902
	36677302, 38853970, 42459583, 50525758,
	//for 0x9a4eb75fc8db5680497ac33fd689b536334292b0
	36816616, 38430645, 41639657, 56951656,
	//for 0xae6bde77bc386d2cb6492f824ded9147d0926512
	37056299, 42138348, 50287799, 80356659,
	//for 0xef470c3a63343585651808b8187bba0e277bc3c8
	37682064, 49512860, 52081898, 56449325,
	//for 0x133d604a2a138f04db8fb7d1f57fd739ad4b08aa
	39602464, 42479918, 44774842, 61731113,
	//for 0x6ea97c1d1588c589589fa0e1f66457897fa9b1cc
	43635343, 44574635, 47946932, 49779509, 51389901, 56944193,
	//for 0x723c1b86c78a04c4f125df4573acb0625bfc69a5
	80530396,

	//contract patch
	53812868,
}

func NeedDebug(chainId string, blockNr uint64) bool {

	if chainId == "pchain" && blockNr == 13311677{
		return true
	}

	if chainId == "child_0" {
		for _, block := range debugBlocks {
			if block == blockNr {
				return true
			}
		}
	}
	return false
}

var ExtractAddrs = map[Address]*struct{} {
	HexToAddress("0x41e0a683ee564f73f1ca240d35e30a71a3848c14") : nil,
	HexToAddress("0x6c665834015f725bd27fecda2394806be3b8d6d9") : nil,
	HexToAddress("0xb6a43df609168ab41af7964fa8cafdeb28a15387") : nil,
	HexToAddress("0xd06b6c1299c04b71febc1e6951458ebdd5a04f7d") : nil,
}

func NeedDebug1(chainId string, addr Address) bool {

	if chainId == "child_0" {
		if _, exist := ExtractAddrs[addr]; exist {
			return true
		}
	}
	return false
}

func init() {
	//bhMap[HexToHash("3d728590790b3955980c5ac99075bf65775aabc2fbaf65a58ce51b5384d1f5b7")] = HexToHash("4554d6932ed514e180cf7c52f5b68e1b5bcd16ecd789242bc088d563905863a5")
}
