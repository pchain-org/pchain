package core

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"io"
	"io/ioutil"
)

// WriteGenesisBlock writes the genesis block to the database as block number 0
func WriteGenesisBlock(chainDb ethdb.Database, reader io.Reader) (*types.Block, error) {
	contents, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var genesis = Genesis{}

	if err := json.Unmarshal(contents, &genesis); err != nil {
		return nil, err
	}

	return SetupGenesisBlockEx(chainDb, &genesis)
}

func SetupGenesisBlockEx(db ethdb.Database, genesis *Genesis) (*types.Block, error) {

	if genesis != nil && genesis.Config == nil {
		return nil, errGenesisNoConfig
	}

	var block *types.Block = nil
	var err error = nil

	// Just commit the new block if there is no stored genesis block.
	stored := GetCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			log.Info("Writing default main-net genesis block")
			genesis = DefaultGenesisBlock()
		} else {
			log.Info("Writing custom genesis block")
		}
		block, err = genesis.Commit(db)
		return block, err
	}

	// Check whether the genesis block is already written.
	if genesis != nil {
		block = genesis.ToBlock(nil)
		hash := block.Hash()
		if hash != stored {
			return nil, &GenesisMismatchError{stored, hash}
		}
	}

	// Get the existing chain configuration.
	newcfg := genesis.configOrDefault(stored)
	storedcfg, err := GetChainConfig(db, stored)
	if err != nil {
		if err == ErrChainConfigNotFound {
			// This case happens if a genesis write was interrupted.
			log.Warn("Found genesis block without chain config")
			err = WriteChainConfig(db, stored, newcfg)
		}
		return block, err
	}
	// Special case: don't change the existing config of a non-mainnet chain if no new
	// config is supplied. These chains would get AllProtocolChanges (and a compat error)
	// if we just continued here.
	if genesis == nil && stored != params.MainnetGenesisHash {
		return block, nil
	}

	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	height := GetBlockNumber(db, GetHeadHeaderHash(db))
	if height == missingNumber {
		return nil, fmt.Errorf("missing block number for head header hash")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, height)
	if compatErr != nil && height != 0 && compatErr.RewindTo != 0 {
		return nil, compatErr
	}
	return block, err
}

// SetupGenesisBlock writes or updates the genesis block in db.
// The block that will be used is:
//
//                          genesis == nil       genesis != nil
//                       +------------------------------------------
//     db has no genesis |  main-net default  |  genesis
//     db has genesis    |  from DB           |  genesis (if compatible)
//
// The stored chain configuration will be updated if it is compatible (i.e. does not
// specify a fork block below the local head block). In case of a conflict, the
// error is a *params.ConfigCompatError and the new, unwritten config is returned.
//
// The returned chain configuration is never nil.
func SetupGenesisBlockWithDefault(db ethdb.Database, genesis *Genesis, isTestnet bool) (*params.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		return params.AllEthashProtocolChanges, common.Hash{}, errGenesisNoConfig
	}

	// Just commit the new block if there is no stored genesis block.
	stored := GetCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			log.Info("Writing default main-net genesis block")
			if isTestnet {
				genesis = DefaultGenesisBlockFromJson(DefaultTestnetGenesisJSON)
			} else {
				genesis = DefaultGenesisBlockFromJson(DefaultMainnetGenesisJSON)
			}
		} else {
			log.Info("Writing custom genesis block")
		}
		block, err := genesis.Commit(db)
		return genesis.Config, block.Hash(), err
	}

	// Check whether the genesis block is already written.
	if genesis != nil {
		hash := genesis.ToBlock(nil).Hash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
	}

	// Get the existing chain configuration.
	newcfg := genesis.configOrDefault(stored)
	storedcfg, err := GetChainConfig(db, stored)
	if err != nil {
		if err == ErrChainConfigNotFound {
			// This case happens if a genesis write was interrupted.
			log.Warn("Found genesis block without chain config")
			err = WriteChainConfig(db, stored, newcfg)
		}
		return newcfg, stored, err
	}
	// Special case: don't change the existing config of a non-mainnet chain if no new
	// config is supplied. These chains would get AllProtocolChanges (and a compat error)
	// if we just continued here.
	if genesis == nil && stored != params.MainnetGenesisHash {
		return storedcfg, stored, nil
	}

	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	height := GetBlockNumber(db, GetHeadHeaderHash(db))
	if height == missingNumber {
		return newcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, height)
	if compatErr != nil && height != 0 && compatErr.RewindTo != 0 {
		return newcfg, stored, compatErr
	}
	return newcfg, stored, WriteChainConfig(db, stored, newcfg)
}

// DefaultGenesisBlock returns the Ethereum main net genesis block.
func DefaultGenesisBlockFromJson(genesisJson string) *Genesis {

	var genesis = Genesis{}

	if err := json.Unmarshal([]byte(genesisJson), &genesis); err != nil {
		return nil
	}

	return &genesis
}

var DefaultMainnetGenesisJSON = `{
	"config": {
		"pChainId": "pchain",
		"chainId": 24160843454325667600331855523506733810605584168331177014437733538279768116753,
		"homesteadBlock": 0,
		"eip150Block": 0,
		"eip150Hash": "0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0",
		"eip155Block": 0,
		"eip158Block": 0,
		"byzantiumBlock": 0,
		"tendermint": {
			"epoch": 30000,
			"policy": 0
		}
	},
	"nonce": "0xdeadbeefdeadbeef",
	"timestamp": "0x0",
	"extraData": "0x307830",
	"gasLimit": "0x8000000",
	"difficulty": "0x400",
	"mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
	"coinbase": "0xf84634254ea1189516e66d9b007760092d9b8922",
	"alloc": {
		"fbd64f3e2b9a40e3892e79af615f76e15f772440": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x898ad220390e380000000",
			"proxiedList": {
							"0x3dcc1290132129c2095327afefccba91bff27f41": "0x30000"
							},
			"candidate": true,
			"commission": 10
		},
		"49fab03ffaa398057507e0c08228315af30b55ea": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x62a1310adfc7f40000000"
		},
		"b62073fd7055f30fd883d950651377c52158ad44": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x8459520ab06af00000000"
		},
		"826b6e6a868a62508989666fa80f4b099a36f951": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x493ca50db0fcac0000000"
		},
		"9db7b33f775b96d12a252e7eb10cfc643c5ea3fd": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x5d48e40a0bf8c00000000"
		},
		"9632ea925d39fda19146fceb646b38ec66a5993a": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x4ef075355c96700000000"
		},
		"3839d4958a8d930101eccb8089bcd2d9ee9c597e": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x3796274caf64c80000000"
		},
		"8035a9b057055d4d117b3aba071e9db7d6055412": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x32eb01744459de0000000"
		},
		"3dcc1290132129c2095327afefccba91bff27f41": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x0",
			"delegate": "0x30000"
		},
		"bb1a4b186d7e64000c4455e1f10ad5e9b12dbba5": {
			"balance": "0x999999999999999999999999999",
			"amount": "0x0"
		}
	}
}`

var DefaultTestnetGenesisJSON = `{
        "config": {
                "pChainId": "testnet",
                "chainId": 98411113441374360242664033072086975431386585974419604025805951356851497696398,
                "homesteadBlock": 0,
                "eip150Block": 0,
                "eip150Hash": "0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0",
                "eip155Block": 0,
                "eip158Block": 0,
                "byzantiumBlock": 0,
                "tendermint": {
                        "epoch": 30000,
                        "policy": 0
                }
        },
        "nonce": "0xdeadbeefdeadbeef",
        "timestamp": "0x0",
        "extraData": "0x307830",
        "gasLimit": "0x8000000",
        "difficulty": "0x400",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0xf84634254ea1189516e66d9b007760092d9b8922",
        "alloc": {
                "05f256d2d5d512c59ba09b56a4fb202e5c883268": {
                        "balance": "0xffd09bead87c0378d8e6400000000",
                        "amount": "0x2a5a058fc295ec000000"
                },
                "3d0e1a7a7674164acf29085b101b000a8a109cc1": {
                        "balance": "0xffd09bead87c0378d8e6400000000",
                        "amount": "0x2a5a058fc295ec000000"
                },
                "4cacbcbf218679dcc9574a90a2061bca4a8d8b6c": {
                        "balance": "0xffd09bead87c0378d8e6400000000",
                        "amount": "0x2a5a058fc295ec000000"
                },
                "50ca5341dfe4b07c41854ff79bdb8ab4e11c996d": {
                        "balance": "0xffd09bead87c0378d8e6400000000",
                        "amount": "0x2a5a058fc295ec000000"
                },
                "79cd31b59e3faab6deea68fbbaafa4da748bbdf6": {
                        "balance": "0xffd09bead87c0378d8e6400000000",
                        "amount": "0x2a5a058fc295ec000000"
                },
                "b3c925a77a24d7c92ec5c06719fd76b51cf6809b": {
                        "balance": "0xffd09bead87c0378d8e6400000000",
                        "amount": "0x2a5a058fc295ec000000"
                },
                "c6179a651918888251380a4e3fee6af81cf091d1": {
                        "balance": "0xffd09bead87c0378d8e6400000000",
                        "amount": "0x2a5a058fc295ec000000"
                },
                "d2fd09246e2ced295411f1f863e6e7b5929bcc59": {
                        "balance": "0xffd09bead87c0378d8e6400000000",
                        "amount": "0x2a5a058fc295ec000000"
                }
        },
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000"
}
`
