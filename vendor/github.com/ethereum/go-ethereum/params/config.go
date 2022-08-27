// Copyright 2016 The go-ethereum Authors
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

package params

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

var (
	MainnetGenesisHash = common.HexToHash("0x5b0937e8c6189a45637f0eeb5d2c62b3794e08b695d1f3e339122c80ff7404e3") // Mainnet genesis hash to enforce below configs on
	TestnetGenesisHash = common.HexToHash("0x5b0937e8c6189a45637f0eeb5d2c62b3794e08b695d1f3e339122c80ff7404e3") // Testnet genesis hash to enforce below configs on

	MainnetOutOfStorageBlock = big.NewInt(5890000)
	TestnetOutOfStorageBlock = big.NewInt(10)

	MainnetChild0OutOfStorageBlock = big.NewInt(13930000)
	TestnetChild0OutOfStorageBlock = big.NewInt(10)

	//From epoch after this block, delegators need to query/retrieve his/her reward by RPC APIs.
	// Note: it does not work exactly from this block, it works from the next epoch
	//       and this number is the main chain block number
	MainnetExtractRewardMainBlock = big.NewInt(9383000)
	TestnetExtractRewardMainBlock = big.NewInt(40)

	//use SaveData2MainBlock v1; which reports epoch/tx3 to main block
	MainnetSd2mcV1MainBlock = big.NewInt(11824000)
	TestnetSd2mcV1MainBlock = big.NewInt(40)

	MainnetSd2mcWhenEpochEndsBlock = big.NewInt(14486667)
	TestnetSd2mcWhenEpochEndsBlock = big.NewInt(40)

	MainnetValidateHTLCBlock = big.NewInt(16000000)
	TestnetValidateHTLCBlock = big.NewInt(40)

	MainnetHeaderHashWithoutTimeBlock = big.NewInt(17160000)
	TestnetHeaderHashWithoutTimeBlock = big.NewInt(40)

	MainnetExtractRewardPatchMainBlock = big.NewInt(20970000)
	TestnetExtractRewardPatchMainBlock = big.NewInt(40)

	MainnetIstanbulBlock = big.NewInt(24195000)
	TestnetIstanbulBlock = big.NewInt(40)

	MainnetMuirGlacierBlock *big.Int = nil //big.NewInt(100000000000)
	TestnetMuirGlacierBlock *big.Int = nil //big.NewInt(100000000000)

	MainnetBerlinBlock *big.Int = nil //big.NewInt(100000000000)
	TestnetBerlinBlock *big.Int = nil //big.NewInt(100000000000)

	MainnetLondonBlock *big.Int = nil
	TestnetLondonBlock *big.Int = nil

	//To patch EvmCatchup commit '5de7c68' in master branch, which removes the check for chainId in tx,
	//must be less than MainnetMuirGlacierBlock, MainnetBerlinBlock and MainnetLondonBlock if they are enabled
	EIP155PatchStartBlock = big.NewInt(41168974)
	EIP155PatchEndBlock   = big.NewInt(41168974)

	MainnetAddExPCBlock = big.NewInt(100000000000)
	TestnetAddExPCBlock = big.NewInt(40)
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		PChainId:                     "pchain",
		ChainId:                      big.NewInt(1),
		HomesteadBlock:               big.NewInt(0),
		DAOForkBlock:                 nil,
		DAOForkSupport:               false,
		EIP150Block:                  big.NewInt(0),
		EIP150Hash:                   common.HexToHash("0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0"),
		EIP155Block:                  big.NewInt(0),
		EIP158Block:                  big.NewInt(0),
		ByzantiumBlock:               big.NewInt(0), //let's start from 1 block
		ConstantinopleBlock:          big.NewInt(0),
		PetersburgBlock:              big.NewInt(0),
		IstanbulBlock:                MainnetIstanbulBlock,
		MuirGlacierBlock:             MainnetMuirGlacierBlock,
		BerlinBlock:                  MainnetBerlinBlock,
		LondonBlock:                  MainnetLondonBlock,
		Child0HashTimeLockContract:   common.HexToAddress("0x18c496af47eb1c0946f64a25d3f589f71934bf3d"),
		OutOfStorageBlock:            MainnetOutOfStorageBlock,
		Child0OutOfStorageBlock:      MainnetChild0OutOfStorageBlock,
		ExtractRewardMainBlock:       MainnetExtractRewardMainBlock,
		ExtractRewardPatchMainBlock:  MainnetExtractRewardPatchMainBlock,
		Sd2mcV1Block:                 MainnetSd2mcV1MainBlock,
		ChildSd2mcWhenEpochEndsBlock: MainnetSd2mcWhenEpochEndsBlock,
		ValidateHTLCBlock:            MainnetValidateHTLCBlock,
		HeaderHashWithoutTimeBlock:   MainnetHeaderHashWithoutTimeBlock,

		Tendermint: &TendermintConfig{
			Epoch:          30000,
			ProposerPolicy: 0,
		},
	}

	// TestnetChainConfig contains the chain parameters to run a node on the test network.
	TestnetChainConfig = &ChainConfig{
		PChainId:                     "testnet",
		ChainId:                      big.NewInt(2),
		HomesteadBlock:               big.NewInt(0),
		DAOForkBlock:                 nil,
		DAOForkSupport:               true,
		EIP150Block:                  big.NewInt(0),
		EIP150Hash:                   common.HexToHash("0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d"),
		EIP155Block:                  big.NewInt(0),
		EIP158Block:                  big.NewInt(0),
		ByzantiumBlock:               big.NewInt(0),
		ConstantinopleBlock:          big.NewInt(0),
		PetersburgBlock:              big.NewInt(0),
		IstanbulBlock:                TestnetIstanbulBlock,
		MuirGlacierBlock:             TestnetMuirGlacierBlock,
		BerlinBlock:                  TestnetBerlinBlock,
		LondonBlock:                  TestnetLondonBlock,
		Child0HashTimeLockContract:   common.HexToAddress("0x0429658b97a75f7160ca551f72b6f85d6fa10439"),
		OutOfStorageBlock:            TestnetOutOfStorageBlock,
		Child0OutOfStorageBlock:      TestnetChild0OutOfStorageBlock,
		ExtractRewardMainBlock:       TestnetExtractRewardMainBlock,
		ExtractRewardPatchMainBlock:  TestnetExtractRewardPatchMainBlock,
		Sd2mcV1Block:                 TestnetSd2mcV1MainBlock,
		ChildSd2mcWhenEpochEndsBlock: TestnetSd2mcWhenEpochEndsBlock,
		ValidateHTLCBlock:            TestnetValidateHTLCBlock,
		HeaderHashWithoutTimeBlock:   TestnetHeaderHashWithoutTimeBlock,
		Tendermint: &TendermintConfig{
			Epoch:          30000,
			ProposerPolicy: 0,
		},
	}

	// RinkebyChainConfig contains the chain parameters to run a node on the Rinkeby test network.
	RinkebyChainConfig = &ChainConfig{
		PChainId:            "",
		ChainId:             big.NewInt(4),
		HomesteadBlock:      big.NewInt(1),
		DAOForkBlock:        nil,
		DAOForkSupport:      true,
		EIP150Block:         big.NewInt(2),
		EIP150Hash:          common.HexToHash("0x9b095b36c15eaf13044373aef8ee0bd3a382a5abb92e402afa44b8249c3a90e9"),
		EIP155Block:         big.NewInt(3),
		EIP158Block:         big.NewInt(3),
		ByzantiumBlock:      big.NewInt(1035301),
		ConstantinopleBlock: big.NewInt(3660663),
		PetersburgBlock:     big.NewInt(4321234),
		IstanbulBlock:       big.NewInt(5435345),
		Clique: &CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	// OttomanChainConfig contains the chain parameters to run a node on the Ottoman test network.
	OttomanChainConfig = &ChainConfig{
		PChainId:            "",
		ChainId:             big.NewInt(5),
		HomesteadBlock:      big.NewInt(1),
		DAOForkBlock:        nil,
		DAOForkSupport:      true,
		EIP150Block:         big.NewInt(2),
		EIP150Hash:          common.HexToHash("0x9b095b36c15eaf13044373aef8ee0bd3a382a5abb92e402afa44b8249c3a90e9"),
		EIP155Block:         big.NewInt(3),
		EIP158Block:         big.NewInt(3),
		ByzantiumBlock:      big.NewInt(math.MaxInt64), // Don't enable yet
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(1561651),
		Istanbul: &IstanbulConfig{
			Epoch:          30000,
			ProposerPolicy: 0,
		},
	}

	// AllEthashProtocolChanges contains every protocol change (EIPs) introduced
	// and accepted by the Ethereum core developers into the Ethash consensus.
	//
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	AllEthashProtocolChanges = &ChainConfig{"", big.NewInt(1337), big.NewInt(0), nil, false, big.NewInt(0), common.Hash{}, big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), common.Address{}, nil, nil, nil, nil, common.Address{}, nil, nil, nil, nil, nil, new(EthashConfig), nil, nil, nil, nil}

	// AllCliqueProtocolChanges contains every protocol change (EIPs) introduced
	// and accepted by the Ethereum core developers into the Clique consensus.
	//
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	AllCliqueProtocolChanges = &ChainConfig{"", big.NewInt(1337), big.NewInt(0), nil, false, big.NewInt(0), common.Hash{}, big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), common.Address{}, nil, nil, nil, nil, common.Address{}, nil, nil, nil, nil, nil, nil, &CliqueConfig{Period: 0, Epoch: 30000}, nil, nil, nil}

	TestChainConfig = &ChainConfig{"", big.NewInt(1), big.NewInt(0), nil, false, big.NewInt(0), common.Hash{}, big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), common.Address{}, nil, nil, nil, nil, common.Address{}, nil, nil, nil, nil, nil, new(EthashConfig), nil, nil, nil, nil}
	TestRules       = TestChainConfig.Rules(new(big.Int))
)

func init() {
	digest := crypto.Keccak256([]byte(MainnetChainConfig.PChainId))
	MainnetChainConfig.ChainId = new(big.Int).SetBytes(digest[:])
}

// TrustedCheckpoint represents a set of post-processed trie roots (CHT and
// BloomTrie) associated with the appropriate section index and head hash. It is
// used to start light syncing from this checkpoint and avoid downloading the
// entire header chain while still being able to securely access old headers/logs.
type TrustedCheckpoint struct {
	SectionIndex uint64      `json:"sectionIndex"`
	SectionHead  common.Hash `json:"sectionHead"`
	CHTRoot      common.Hash `json:"chtRoot"`
	BloomRoot    common.Hash `json:"bloomRoot"`
}

// HashEqual returns an indicator comparing the itself hash with given one.
func (c *TrustedCheckpoint) HashEqual(hash common.Hash) bool {
	if c.Empty() {
		return hash == common.Hash{}
	}
	return c.Hash() == hash
}

// Hash returns the hash of checkpoint's four key fields(index, sectionHead, chtRoot and bloomTrieRoot).
func (c *TrustedCheckpoint) Hash() common.Hash {
	var sectionIndex [8]byte
	binary.BigEndian.PutUint64(sectionIndex[:], c.SectionIndex)

	w := sha3.NewLegacyKeccak256()
	w.Write(sectionIndex[:])
	w.Write(c.SectionHead[:])
	w.Write(c.CHTRoot[:])
	w.Write(c.BloomRoot[:])

	var h common.Hash
	w.Sum(h[:0])
	return h
}

// Empty returns an indicator whether the checkpoint is regarded as empty.
func (c *TrustedCheckpoint) Empty() bool {
	return c.SectionHead == (common.Hash{}) || c.CHTRoot == (common.Hash{}) || c.BloomRoot == (common.Hash{})
}

// CheckpointOracleConfig represents a set of checkpoint contract(which acts as an oracle)
// config which used for light client checkpoint syncing.
type CheckpointOracleConfig struct {
	Address   common.Address   `json:"address"`
	Signers   []common.Address `json:"signers"`
	Threshold uint64           `json:"threshold"`
}

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig struct {
	PChainId string   `json:"pChainId"` //PChain id identifies the current chain
	ChainId  *big.Int `json:"chainId"`  // Chain id identifies the current chain and is used for replay protection

	HomesteadBlock *big.Int `json:"homesteadBlock,omitempty"` // Homestead switch block (nil = no fork, 0 = already homestead)

	DAOForkBlock   *big.Int `json:"daoForkBlock,omitempty"`   // TheDAO hard-fork switch block (nil = no fork)
	DAOForkSupport bool     `json:"daoForkSupport,omitempty"` // Whether the nodes supports or opposes the DAO hard-fork

	// EIP150 implements the Gas price changes (https://github.com/ethereum/EIPs/issues/150)
	EIP150Block *big.Int    `json:"eip150Block,omitempty"` // EIP150 HF block (nil = no fork)
	EIP150Hash  common.Hash `json:"eip150Hash,omitempty"`  // EIP150 HF hash (needed for header only clients as only gas pricing changed)

	EIP155Block *big.Int `json:"eip155Block,omitempty"` // EIP155 HF block
	EIP158Block *big.Int `json:"eip158Block,omitempty"` // EIP158 HF block

	ByzantiumBlock      *big.Int `json:"byzantiumBlock,omitempty"`      // Byzantium switch block (nil = no fork, 0 = already on byzantium)
	ConstantinopleBlock *big.Int `json:"constantinopleBlock,omitempty"` // Constantinople switch block (nil = no fork, 0 = already activated)
	PetersburgBlock     *big.Int `json:"petersburgBlock,omitempty"`     // Petersburg switch block (nil = same as Constantinople)
	IstanbulBlock       *big.Int `json:"istanbulBlock,omitempty"`       // Istanbul switch block (nil = no fork, 0 = already on istanbul)
	MuirGlacierBlock    *big.Int `json:"muirGlacierBlock,omitempty"`    // Eip-2384 (bomb delay) switch block (nil = no fork, 0 = already activated)
	BerlinBlock         *big.Int `json:"berlinBlock,omitempty"`         // Berlin switch block (nil = no fork, 0 = already on berlin)
	LondonBlock         *big.Int `json:"londonBlock,omitempty"`         // London switch block (nil = no fork, 0 = already on london)

	// PCHAIN HordFork
	HashTimeLockContract        common.Address `json:"htlc,omitempty"`         // Hash Time Lock Contract Address
	OutOfStorageBlock           *big.Int       `json:"oosBlock,omitempty"`     // Out of storage HardFork block
	ExtractRewardMainBlock      *big.Int       `json:"erBlock,omitempty"`      // Extract reward HardFork block
	ExtractRewardPatchMainBlock *big.Int       `json:"erPatchBlock,omitempty"` // Extract reward Patch HardFork block
	Sd2mcV1Block                *big.Int       `json:"sd2mcV1Block,omitempty"`

	// For default setup propose
	Child0HashTimeLockContract   common.Address `json:"child0HashTimeLockContract,omitempty"`
	Child0OutOfStorageBlock      *big.Int       `json:"child0OutOfStorageBlock,omitempty"`
	ChildSd2mcWhenEpochEndsBlock *big.Int       `json:"childSd2mcWhenEpochEndsBlock,omitempty"`
	ValidateHTLCBlock            *big.Int       `json:"validateHTLCBlock,omitempty"`
	HeaderHashWithoutTimeBlock   *big.Int       `json:"headerHashWithoutTimeBlock,omitempty"`
	MarkProposedInEpochMainBlock *big.Int       `json:"markProposedInEpochMainBlock,omitempty"`

	// Various consensus engines
	Ethash     *EthashConfig     `json:"ethash,omitempty"`
	Clique     *CliqueConfig     `json:"clique,omitempty"`
	Istanbul   *IstanbulConfig   `json:"istanbul,omitempty"`
	Tendermint *TendermintConfig `json:"tendermint,omitempty"`

	ChainLogger log.Logger `json:"-"`
}

// EthashConfig is the consensus engine configs for proof-of-work based sealing.
type EthashConfig struct{}

// String implements the stringer interface, returning the consensus engine details.
func (c *EthashConfig) String() string {
	return "ethash"
}

// CliqueConfig is the consensus engine configs for proof-of-authority based sealing.
type CliqueConfig struct {
	Period uint64 `json:"period"` // Number of seconds between blocks to enforce
	Epoch  uint64 `json:"epoch"`  // Epoch length to reset votes and checkpoint
}

// String implements the stringer interface, returning the consensus engine details.
func (c *CliqueConfig) String() string {
	return "clique"
}

// IstanbulConfig is the consensus engine configs for Istanbul based sealing.
type IstanbulConfig struct {
	Epoch          uint64 `json:"epoch"`  // Epoch length to reset votes and checkpoint
	ProposerPolicy uint64 `json:"policy"` // The policy for proposer selection
}

// TendermintConfig is the consensus engine configs for Istanbul based sealing.
type TendermintConfig struct {
	Epoch          uint64 `json:"epoch"`  // Epoch length to reset votes and checkpoint
	ProposerPolicy uint64 `json:"policy"` // The policy for proposer selection
}

// String implements the stringer interface, returning the consensus engine details.
func (c *IstanbulConfig) String() string {
	return "istanbul"
}

// String implements the stringer interface, returning the consensus engine details.
func (c *TendermintConfig) String() string {
	return "tendermint"
}

// Create a new Chain Config based on the Chain ID, for child chain creation purpose
func NewChildChainConfig(childChainID string) *ChainConfig {
	config := &ChainConfig{
		PChainId:            childChainID,
		HomesteadBlock:      big.NewInt(0),
		DAOForkBlock:        nil,
		DAOForkSupport:      false,
		EIP150Block:         big.NewInt(0),
		EIP150Hash:          common.HexToHash("0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0"),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0), //let's start from block 0
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		Tendermint: &TendermintConfig{
			Epoch:          30000,
			ProposerPolicy: 0,
		},
	}

	digest := crypto.Keccak256([]byte(config.PChainId))
	config.ChainId = new(big.Int).SetBytes(digest[:])

	return config
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	var engine interface{}
	switch {
	case c.Ethash != nil:
		engine = c.Ethash
	case c.Clique != nil:
		engine = c.Clique
	case c.Istanbul != nil:
		engine = c.Istanbul
	case c.Tendermint != nil:
		engine = c.Tendermint
	default:
		engine = "unknown"
	}
	return fmt.Sprintf("{PChainId: %s ChainID: %v Homestead: %v DAO: %v DAOSupport: %v EIP150: %v EIP155: %v EIP158: %v Byzantium: %v Constantinople: %v Petersburg: %v Istanbul: %v, MuirGlacier: %v, Berlin: %v,London:%v, Engine: %v}",
		c.PChainId,
		c.ChainId,
		c.HomesteadBlock,
		c.DAOForkBlock,
		c.DAOForkSupport,
		c.EIP150Block,
		c.EIP155Block,
		c.EIP158Block,
		c.ByzantiumBlock,
		c.ConstantinopleBlock,
		c.PetersburgBlock,
		c.IstanbulBlock,
		c.MuirGlacierBlock,
		c.BerlinBlock,
		c.LondonBlock,
		engine,
	)
}

// IsHomestead returns whether num is either equal to the homestead block or greater.
func (c *ChainConfig) IsHomestead(num *big.Int) bool {
	return isForked(c.HomesteadBlock, num)
}

// IsDAOFork returns whether num is either equal to the DAO fork block or greater.
func (c *ChainConfig) IsDAOFork(num *big.Int) bool {
	return isForked(c.DAOForkBlock, num)
}

// IsEIP150 returns whether num is either equal to the EIP150 fork block or greater.
func (c *ChainConfig) IsEIP150(num *big.Int) bool {
	return isForked(c.EIP150Block, num)
}

// IsEIP155 returns whether num is either equal to the EIP155 fork block or greater.
func (c *ChainConfig) IsEIP155(num *big.Int) bool {
	return isForked(c.EIP155Block, num)
}

// IsEIP158 returns whether num is either equal to the EIP158 fork block or greater.
func (c *ChainConfig) IsEIP158(num *big.Int) bool {
	return isForked(c.EIP158Block, num)
}

// IsByzantium returns whether num is either equal to the Byzantium fork block or greater.
func (c *ChainConfig) IsByzantium(num *big.Int) bool {
	return isForked(c.ByzantiumBlock, num)
}

// IsConstantinople returns whether num is either equal to the Constantinople fork block or greater.
func (c *ChainConfig) IsConstantinople(num *big.Int) bool {
	return isForked(c.ConstantinopleBlock, num)
}

// IsMuirGlacier returns whether num is either equal to the Muir Glacier (EIP-2384) fork block or greater.
func (c *ChainConfig) IsMuirGlacier(num *big.Int) bool {
	return isForked(c.MuirGlacierBlock, num)
}

// IsPetersburg returns whether num is either
// - equal to or greater than the PetersburgBlock fork block,
// - OR is nil, and Constantinople is active
func (c *ChainConfig) IsPetersburg(num *big.Int) bool {
	return isForked(c.PetersburgBlock, num) || c.PetersburgBlock == nil && isForked(c.ConstantinopleBlock, num)
}

// IsIstanbul returns whether num is either equal to the Istanbul fork block or greater.
func (c *ChainConfig) IsIstanbul(num *big.Int) bool {
	return isForked(c.IstanbulBlock, num)
}

// IsBerlin returns whether num is either equal to the Berlin fork block or greater.
func (c *ChainConfig) IsBerlin(num *big.Int) bool {
	return isForked(c.BerlinBlock, num)
}

// IsLondon returns whether num is either equal to the London fork block or greater.
func (c *ChainConfig) IsLondon(num *big.Int) bool {
	return isForked(c.LondonBlock, num)
}

func (c *ChainConfig) IsEWASM(num *big.Int) bool {
	return false
}

func (c *ChainConfig) IsHashTimeLockWithdraw(num *big.Int, contractAddress *common.Address, withdraw []byte) bool {
	return contractAddress != nil && c.HashTimeLockContract == *contractAddress && len(withdraw) > 4 && bytes.Equal(withdraw[:4], common.Hex2Bytes("63615149"))
}

func (c *ChainConfig) IsOutOfStorage(blockNumber, mainBlockNumber *big.Int) bool {

	log.Debugf("IsOutOfStorage, c.PChainId, c.OutOfStorageBlock, blockNumber, mainBlockNumber is %v, %v, %v, %v",
		c.PChainId, c.OutOfStorageBlock, blockNumber, mainBlockNumber)
	if c.PChainId == "child_0" || c.IsMainChain() {
		return isForked(c.OutOfStorageBlock, blockNumber)
	} else {
		return isForked(c.OutOfStorageBlock, mainBlockNumber)
	}
}

func (c *ChainConfig) IsSelfRetrieveReward(mainBlockNumber *big.Int) bool {
	return isForked(c.ExtractRewardMainBlock, mainBlockNumber)
}

func (c *ChainConfig) IsSd2mcV1(mainBlockNumber *big.Int) bool {
	return isForked(c.Sd2mcV1Block, mainBlockNumber)
}

func (c *ChainConfig) IsChildSd2mcWhenEpochEndsBlock(mainBlockNumber *big.Int) bool {
	return isForked(c.ChildSd2mcWhenEpochEndsBlock, mainBlockNumber)
}

func IsSelfRetrieveReward(mainChainId string, mainBlockNumber *big.Int) bool {
	if mainChainId == MainnetChainConfig.PChainId {
		return isForked(MainnetExtractRewardMainBlock, mainBlockNumber)
	} else if mainChainId == TestnetChainConfig.PChainId {
		return isForked(TestnetExtractRewardMainBlock, mainBlockNumber)
	}
	return false
}

func (c *ChainConfig) IsSelfRetrieveRewardPatch(blockNumber, mainBlockNumber *big.Int) bool {
	if c.IsMainChain() {
		return isForked(c.ExtractRewardPatchMainBlock, blockNumber)
	} else {
		return isForked(c.ExtractRewardPatchMainBlock, mainBlockNumber)
	}
}

func IsSd2mc(mainChainId string, mainBlockNumber *big.Int) bool {
	if mainChainId == MainnetChainConfig.PChainId {
		return isForked(MainnetSd2mcV1MainBlock, mainBlockNumber)
	} else if mainChainId == TestnetChainConfig.PChainId {
		return isForked(TestnetSd2mcV1MainBlock, mainBlockNumber)
	}
	return false
}

func (c *ChainConfig) CeaseValidateHashTimeLockContract(mainBlockNumber *big.Int) bool {
	return isForked(c.ValidateHTLCBlock, mainBlockNumber)
}

func (c *ChainConfig) IsHeaderHashWithoutTimeBlock(mainBlockNumber *big.Int) bool {
	return isForked(c.HeaderHashWithoutTimeBlock, mainBlockNumber)
}

func (c *ChainConfig) IsMarkProposedInEpoch(mainBlockNumber *big.Int) bool {
	return isForked(c.MarkProposedInEpochMainBlock, mainBlockNumber)
}

// Check whether is on main chain or not
func (c *ChainConfig) IsMainChain() bool {
	return c.PChainId == MainnetChainConfig.PChainId || c.PChainId == TestnetChainConfig.PChainId
}

// Check provided chain id is on main chain or not
func IsMainChain(chainId string) bool {
	return chainId == MainnetChainConfig.PChainId || chainId == TestnetChainConfig.PChainId
}

// GasTable returns the gas table corresponding to the current phase (homestead or homestead reprice).
//
// The returned GasTable's fields shouldn't, under any circumstances, be changed.
func (c *ChainConfig) GasTable(num *big.Int) GasTable {
	if num == nil {
		return GasTableHomestead
	}
	switch {
	case c.IsEIP158(num):
		return GasTableEIP158
	case c.IsEIP150(num):
		return GasTableEIP150
	default:
		return GasTableHomestead
	}
}

// CheckCompatible checks whether scheduled fork transitions have been imported
// with a mismatching chain configuration.
func (c *ChainConfig) CheckCompatible(newcfg *ChainConfig, height uint64) *ConfigCompatError {
	bhead := new(big.Int).SetUint64(height)

	// Iterate checkCompatible to find the lowest conflict.
	var lasterr *ConfigCompatError
	for {
		err := c.checkCompatible(newcfg, bhead)
		if err == nil || (lasterr != nil && err.RewindTo == lasterr.RewindTo) {
			break
		}
		lasterr = err
		bhead.SetUint64(err.RewindTo)
	}
	return lasterr
}

// CheckConfigForkOrder checks that we don't "skip" any forks, geth isn't pluggable enough
// to guarantee that forks can be implemented in a different order than on official networks
func (c *ChainConfig) CheckConfigForkOrder() error {
	type fork struct {
		name     string
		block    *big.Int
		optional bool // if true, the fork may be nil and next fork is still allowed
	}
	var lastFork fork
	for _, cur := range []fork{
		{name: "homesteadBlock", block: c.HomesteadBlock},
		{name: "daoForkBlock", block: c.DAOForkBlock, optional: true},
		{name: "eip150Block", block: c.EIP150Block},
		{name: "eip155Block", block: c.EIP155Block},
		{name: "eip158Block", block: c.EIP158Block},
		{name: "byzantiumBlock", block: c.ByzantiumBlock},
		{name: "constantinopleBlock", block: c.ConstantinopleBlock},
		{name: "petersburgBlock", block: c.PetersburgBlock},
		{name: "istanbulBlock", block: c.IstanbulBlock},
		{name: "muirGlacierBlock", block: c.MuirGlacierBlock, optional: true},
		{name: "berlinBlock", block: c.BerlinBlock},
		{name: "londonBlock", block: c.LondonBlock},
	} {
		if lastFork.name != "" {
			// Next one must be higher number
			if lastFork.block == nil && cur.block != nil {
				return fmt.Errorf("unsupported fork ordering: %v not enabled, but %v enabled at %v",
					lastFork.name, cur.name, cur.block)
			}
			if lastFork.block != nil && cur.block != nil {
				if lastFork.block.Cmp(cur.block) > 0 {
					return fmt.Errorf("unsupported fork ordering: %v enabled at %v, but %v enabled at %v",
						lastFork.name, lastFork.block, cur.name, cur.block)
				}
			}
		}
		// If it was optional and not set, then ignore it
		if !cur.optional || cur.block != nil {
			lastFork = cur
		}
	}
	return nil
}

func (c *ChainConfig) checkCompatible(newcfg *ChainConfig, head *big.Int) *ConfigCompatError {
	if isForkIncompatible(c.HomesteadBlock, newcfg.HomesteadBlock, head) {
		return newCompatError("Homestead fork block", c.HomesteadBlock, newcfg.HomesteadBlock)
	}
	if isForkIncompatible(c.DAOForkBlock, newcfg.DAOForkBlock, head) {
		return newCompatError("DAO fork block", c.DAOForkBlock, newcfg.DAOForkBlock)
	}
	if c.IsDAOFork(head) && c.DAOForkSupport != newcfg.DAOForkSupport {
		return newCompatError("DAO fork support flag", c.DAOForkBlock, newcfg.DAOForkBlock)
	}
	if isForkIncompatible(c.EIP150Block, newcfg.EIP150Block, head) {
		return newCompatError("EIP150 fork block", c.EIP150Block, newcfg.EIP150Block)
	}
	if isForkIncompatible(c.EIP155Block, newcfg.EIP155Block, head) {
		return newCompatError("EIP155 fork block", c.EIP155Block, newcfg.EIP155Block)
	}
	if isForkIncompatible(c.EIP158Block, newcfg.EIP158Block, head) {
		return newCompatError("EIP158 fork block", c.EIP158Block, newcfg.EIP158Block)
	}
	if c.IsEIP158(head) && !configNumEqual(c.ChainId, newcfg.ChainId) {
		return newCompatError("EIP158 chain ID", c.EIP158Block, newcfg.EIP158Block)
	}
	if isForkIncompatible(c.ByzantiumBlock, newcfg.ByzantiumBlock, head) {
		return newCompatError("Byzantium fork block", c.ByzantiumBlock, newcfg.ByzantiumBlock)
	}
	if isForkIncompatible(c.ConstantinopleBlock, newcfg.ConstantinopleBlock, head) {
		return newCompatError("Constantinople fork block", c.ConstantinopleBlock, newcfg.ConstantinopleBlock)
	}
	if isForkIncompatible(c.PetersburgBlock, newcfg.PetersburgBlock, head) {
		// the only case where we allow Petersburg to be set in the past is if it is equal to Constantinople
		// mainly to satisfy fork ordering requirements which state that Petersburg fork be set if Constantinople fork is set
		if isForkIncompatible(c.ConstantinopleBlock, newcfg.PetersburgBlock, head) {
			return newCompatError("Petersburg fork block", c.PetersburgBlock, newcfg.PetersburgBlock)
		}
	}
	if isForkIncompatible(c.IstanbulBlock, newcfg.IstanbulBlock, head) {
		return newCompatError("Istanbul fork block", c.IstanbulBlock, newcfg.IstanbulBlock)
	}
	if isForkIncompatible(c.MuirGlacierBlock, newcfg.MuirGlacierBlock, head) {
		return newCompatError("Muir Glacier fork block", c.MuirGlacierBlock, newcfg.MuirGlacierBlock)
	}
	if isForkIncompatible(c.BerlinBlock, newcfg.BerlinBlock, head) {
		return newCompatError("Berlin fork block", c.BerlinBlock, newcfg.BerlinBlock)
	}
	if isForkIncompatible(c.LondonBlock, newcfg.LondonBlock, head) {
		return newCompatError("London fork block", c.LondonBlock, newcfg.LondonBlock)
	}
	return nil
}

// isForkIncompatible returns true if a fork scheduled at s1 cannot be rescheduled to
// block s2 because head is already past the fork.
func isForkIncompatible(s1, s2, head *big.Int) bool {
	return (isForked(s1, head) || isForked(s2, head)) && !configNumEqual(s1, s2)
}

// isForked returns whether a fork scheduled at block s is active at the given head block.
func isForked(s, head *big.Int) bool {
	if s == nil || head == nil {
		return false
	}
	return s.Cmp(head) <= 0
}

func configNumEqual(x, y *big.Int) bool {
	if x == nil {
		return y == nil
	}
	if y == nil {
		return x == nil
	}
	return x.Cmp(y) == 0
}

// ConfigCompatError is raised if the locally-stored blockchain is initialised with a
// ChainConfig that would alter the past.
type ConfigCompatError struct {
	What string
	// block numbers of the stored and new configurations
	StoredConfig, NewConfig *big.Int
	// the block number to which the local chain must be rewound to correct the error
	RewindTo uint64
}

func newCompatError(what string, storedblock, newblock *big.Int) *ConfigCompatError {
	var rew *big.Int
	switch {
	case storedblock == nil:
		rew = newblock
	case newblock == nil || storedblock.Cmp(newblock) < 0:
		rew = storedblock
	default:
		rew = newblock
	}
	err := &ConfigCompatError{what, storedblock, newblock, 0}
	if rew != nil && rew.Sign() > 0 {
		err.RewindTo = rew.Uint64() - 1
	}
	return err
}

func (err *ConfigCompatError) Error() string {
	return fmt.Sprintf("mismatching %s in database (have %d, want %d, rewindto %d)", err.What, err.StoredConfig, err.NewConfig, err.RewindTo)
}

// Rules wraps ChainConfig and is merely syntactic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules struct {
	ChainId                                                 *big.Int
	IsHomestead, IsEIP150, IsEIP155, IsEIP158               bool
	IsByzantium, IsConstantinople, IsPetersburg, IsIstanbul bool
	IsBerlin, IsLondon, IsCatalyst                          bool
}

// Rules ensures c's ChainID is not nil.
func (c *ChainConfig) Rules(mainchainNum *big.Int) Rules {
	chainId := c.ChainId
	if chainId == nil {
		chainId = new(big.Int)
	}
	rules := Rules{
		ChainId:          new(big.Int).Set(chainId),
		IsHomestead:      c.IsHomestead(mainchainNum),
		IsEIP150:         c.IsEIP150(mainchainNum),
		IsEIP155:         c.IsEIP155(mainchainNum),
		IsEIP158:         c.IsEIP158(mainchainNum),
		IsByzantium:      c.IsByzantium(mainchainNum),
		IsConstantinople: c.IsConstantinople(mainchainNum),
		IsPetersburg:     c.IsPetersburg(mainchainNum),
		IsIstanbul:       c.IsIstanbul(mainchainNum),
		IsBerlin:         c.IsBerlin(mainchainNum),
		IsLondon:         c.IsLondon(mainchainNum),
	}

	return rules
}
