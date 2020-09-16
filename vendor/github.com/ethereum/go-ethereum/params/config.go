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
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	MainnetGenesisHash = common.HexToHash("0x5b0937e8c6189a45637f0eeb5d2c62b3794e08b695d1f3e339122c80ff7404e3") // Mainnet genesis hash to enforce below configs on
	TestnetGenesisHash = common.HexToHash("0x5b0937e8c6189a45637f0eeb5d2c62b3794e08b695d1f3e339122c80ff7404e3") // Testnet genesis hash to enforce below configs on

	//From epoch after this block, delegators need to query/retrieve his/her reward by RPC APIs.
	// Note: it does not work exactly from this block, it works from the next epoch
	//       and this number is the main chain block number
	MainnetExtractRewardMainBlock = big.NewInt(9383000)
	TestnetExtractRewardMainBlock = big.NewInt(2550000)

	//use SaveData2MainBlock v1; which reports epoch/tx3 to main block
	MainnetSd2mcV1MainBlock = big.NewInt(11824000)
	TestnetSd2mcV1MainBlock = big.NewInt(40)

	MainnetSd2mcWhenEpochEndsBlock = big.NewInt(14486667)
	TestnetSd2mcWhenEpochEndsBlock = big.NewInt(40)

	MainnetValidateHTLCBlock = big.NewInt(16000000)
	TestnetValidateHTLCBlock = big.NewInt(9785000)

)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		PChainId:       "pchain",
		HomesteadBlock: big.NewInt(0),
		DAOForkBlock:   nil,
		DAOForkSupport: false,
		EIP150Block:    big.NewInt(0),
		EIP150Hash:     common.HexToHash("0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0"),
		EIP155Block:    big.NewInt(0),
		EIP158Block:    big.NewInt(0),
		//ByzantiumBlock:      big.NewInt(4370000),
		ByzantiumBlock:               big.NewInt(0), //let's start from 1 block
		ConstantinopleBlock:          nil,
		Child0HashTimeLockContract:   common.HexToAddress("0x18c496af47eb1c0946f64a25d3f589f71934bf3d"),
		OutOfStorageBlock:            big.NewInt(5890000),
		Child0OutOfStorageBlock:      big.NewInt(13930000),
		ExtractRewardMainBlock:       MainnetExtractRewardMainBlock,
		Sd2mcV1Block:                 MainnetSd2mcV1MainBlock,
		ChildSd2mcWhenEpochEndsBlock: MainnetSd2mcWhenEpochEndsBlock,
		ValidateHTLCBlock: MainnetValidateHTLCBlock,

		Tendermint: &TendermintConfig{
			Epoch:          30000,
			ProposerPolicy: 0,
		},
	}

	// TestnetChainConfig contains the chain parameters to run a node on the test network.
	TestnetChainConfig = &ChainConfig{
		PChainId:                   "testnet",
		ChainId:                    big.NewInt(2),
		HomesteadBlock:             big.NewInt(0),
		DAOForkBlock:               nil,
		DAOForkSupport:             true,
		EIP150Block:                big.NewInt(0),
		EIP150Hash:                 common.HexToHash("0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d"),
		EIP155Block:                big.NewInt(10),
		EIP158Block:                big.NewInt(10),
		ByzantiumBlock:             big.NewInt(1700000),
		ConstantinopleBlock:        nil,
		Child0HashTimeLockContract: common.HexToAddress("0x0429658b97a75f7160ca551f72b6f85d6fa10439"),
		OutOfStorageBlock:          big.NewInt(10),
		Child0OutOfStorageBlock:    big.NewInt(10),
		ExtractRewardMainBlock:     TestnetExtractRewardMainBlock,
		Sd2mcV1Block:               TestnetSd2mcV1MainBlock,
		ChildSd2mcWhenEpochEndsBlock: TestnetSd2mcWhenEpochEndsBlock,
		ValidateHTLCBlock: TestnetValidateHTLCBlock,

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
		ConstantinopleBlock: nil,
		Clique: &CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	// OttomanChainConfig contains the chain parameters to run a node on the Ottoman test network.
	OttomanChainConfig = &ChainConfig{
		PChainId:       "",
		ChainId:        big.NewInt(5),
		HomesteadBlock: big.NewInt(1),
		DAOForkBlock:   nil,
		DAOForkSupport: true,
		EIP150Block:    big.NewInt(2),
		EIP150Hash:     common.HexToHash("0x9b095b36c15eaf13044373aef8ee0bd3a382a5abb92e402afa44b8249c3a90e9"),
		EIP155Block:    big.NewInt(3),
		EIP158Block:    big.NewInt(3),
		ByzantiumBlock: big.NewInt(math.MaxInt64), // Don't enable yet

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
	AllEthashProtocolChanges = &ChainConfig{"", big.NewInt(1337), big.NewInt(0), nil, false, big.NewInt(0), common.Hash{}, big.NewInt(0), big.NewInt(0), big.NewInt(0), nil, common.Address{}, nil, nil, nil, common.Address{}, nil, nil,nil,new(EthashConfig), nil, nil, nil, nil}

	// AllCliqueProtocolChanges contains every protocol change (EIPs) introduced
	// and accepted by the Ethereum core developers into the Clique consensus.
	//
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	AllCliqueProtocolChanges = &ChainConfig{"", big.NewInt(1337), big.NewInt(0), nil, false, big.NewInt(0), common.Hash{}, big.NewInt(0), big.NewInt(0), big.NewInt(0), nil, common.Address{}, nil, nil, nil, common.Address{}, nil, nil, nil,nil, &CliqueConfig{Period: 0, Epoch: 30000}, nil, nil, nil}

	TestChainConfig = &ChainConfig{"", big.NewInt(1), big.NewInt(0), nil, false, big.NewInt(0), common.Hash{}, big.NewInt(0), big.NewInt(0), big.NewInt(0), nil, common.Address{}, nil, nil, nil, common.Address{}, nil, nil, nil,new(EthashConfig), nil, nil, nil, nil}
	TestRules       = TestChainConfig.Rules(new(big.Int))
)

func init() {
	digest := crypto.Keccak256([]byte(MainnetChainConfig.PChainId))
	MainnetChainConfig.ChainId = new(big.Int).SetBytes(digest[:])
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

	// PCHAIN HF
	HashTimeLockContract   common.Address `json:"htlc,omitempty"`     // Hash Time Lock Contract Address
	OutOfStorageBlock      *big.Int       `json:"oosBlock,omitempty"` // Out of storage HF block
	ExtractRewardMainBlock *big.Int       `json:"erBlock,omitempty"`  // Extract reward HF block
	Sd2mcV1Block           *big.Int       `json:"sd2mcV1Block, omitempty"`

	// For default setup propose
	Child0HashTimeLockContract   common.Address
	Child0OutOfStorageBlock      *big.Int
	ChildSd2mcWhenEpochEndsBlock *big.Int
	ValidateHTLCBlock *big.Int

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
		PChainId:       childChainID,
		HomesteadBlock: big.NewInt(0),
		DAOForkBlock:   nil,
		DAOForkSupport: false,
		EIP150Block:    big.NewInt(0),
		EIP150Hash:     common.HexToHash("0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0"),
		EIP155Block:    big.NewInt(0),
		EIP158Block:    big.NewInt(0),
		//ByzantiumBlock:      big.NewInt(4370000),
		ByzantiumBlock:      big.NewInt(0), //let's start from 1 block
		ConstantinopleBlock: nil,
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
	return fmt.Sprintf("{PChainId: %s ChainID: %v Homestead: %v DAO: %v DAOSupport: %v EIP150: %v EIP155: %v EIP158: %v Byzantium: %v Constantinople: %v Engine: %v}",
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
		engine,
	)
}

// IsHomestead returns whether num is either equal to the homestead block or greater.
func (c *ChainConfig) IsHomestead(num *big.Int) bool {
	return isForked(c.HomesteadBlock, num)
}

// IsDAO returns whether num is either equal to the DAO fork block or greater.
func (c *ChainConfig) IsDAOFork(num *big.Int) bool {
	return isForked(c.DAOForkBlock, num)
}

func (c *ChainConfig) IsEIP150(num *big.Int) bool {
	return isForked(c.EIP150Block, num)
}

func (c *ChainConfig) IsEIP155(num *big.Int) bool {
	return isForked(c.EIP155Block, num)
}

func (c *ChainConfig) IsEIP158(num *big.Int) bool {
	return isForked(c.EIP158Block, num)
}

func (c *ChainConfig) IsByzantium(num *big.Int) bool {
	return isForked(c.ByzantiumBlock, num)
}

func (c *ChainConfig) IsConstantinople(num *big.Int) bool {
	return isForked(c.ConstantinopleBlock, num)
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

func (c *ChainConfig)IsChildSd2mcWhenEpochEndsBlock(mainBlockNumber *big.Int) bool {
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

func IsSd2mc(mainChainId string, mainBlockNumber *big.Int) bool {
	if mainChainId == MainnetChainConfig.PChainId {
		return isForked(MainnetSd2mcV1MainBlock, mainBlockNumber)
	} else if mainChainId == TestnetChainConfig.PChainId {
		return isForked(TestnetSd2mcV1MainBlock, mainBlockNumber)
	}
	return false
}

func (c *ChainConfig)CeaseValidateHashTimeLockContract(mainBlockNumber *big.Int) bool {
	return isForked(c.ValidateHTLCBlock, mainBlockNumber)
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

// Rules wraps ChainConfig and is merely syntatic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules struct {
	ChainId                                   *big.Int
	IsHomestead, IsEIP150, IsEIP155, IsEIP158 bool
	IsByzantium                               bool
}

func (c *ChainConfig) Rules(num *big.Int) Rules {
	chainId := c.ChainId
	if chainId == nil {
		chainId = new(big.Int)
	}
	return Rules{ChainId: new(big.Int).Set(chainId), IsHomestead: c.IsHomestead(num), IsEIP150: c.IsEIP150(num), IsEIP155: c.IsEIP155(num), IsEIP158: c.IsEIP158(num), IsByzantium: c.IsByzantium(num)}
}
