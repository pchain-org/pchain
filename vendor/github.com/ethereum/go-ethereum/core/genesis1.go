package core

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"io"
	"io/ioutil"
	"math/big"
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
	stored := rawdb.ReadCanonicalHash(db, 0)
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
	storedcfg := rawdb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		rawdb.WriteChainConfig(db, stored, newcfg)
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
	height := rawdb.ReadHeaderNumber(db, rawdb.ReadHeadHeaderHash(db))
	if height == nil {
		return nil, fmt.Errorf("missing block number for head header hash")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, *height)
	if compatErr != nil && *height != 0 && compatErr.RewindTo != 0 {
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
func SetupGenesisBlockWithDefault(db ethdb.Database, genesis *Genesis, isMainChain, isTestnet bool, overrideLondon *big.Int) (*params.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		return params.AllEthashProtocolChanges, common.Hash{}, errGenesisNoConfig
	}

	// Just commit the new block if there is no stored genesis block.
	stored := rawdb.ReadCanonicalHash(db, 0)
	if (stored == common.Hash{} && isMainChain) {
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
	if overrideLondon != nil {
		newcfg.LondonBlock = overrideLondon
	}
	if err := newcfg.CheckConfigForkOrder(); err != nil {
		return newcfg, common.Hash{}, err
	}
	storedcfg := rawdb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		rawdb.WriteChainConfig(db, stored, newcfg)
		return newcfg, stored, nil
	}
	// Special case: don't change the existing config of a non-mainnet chain if no new
	// config is supplied. These chains would get AllProtocolChanges (and a compat error)
	// if we just continued here.
	if genesis == nil && stored != params.MainnetGenesisHash {
		return storedcfg, stored, nil
	}

	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	height := rawdb.ReadHeaderNumber(db, rawdb.ReadHeadHeaderHash(db))
	if height == nil {
		return newcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, *height)
	if compatErr != nil && *height != 0 && compatErr.RewindTo != 0 {
		return newcfg, stored, compatErr
	}
	rawdb.WriteChainConfig(db, stored, newcfg)
	return newcfg, stored, nil
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
		"oosBlock": 5890000,
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
	"coinbase": "0x0000000000000000000000000000000000000000",
	"alloc": {
		"0168647b3d5981d9a0711a0339052a016fb6e1b7": {
			"balance": "0xeed01d82c0a6925ccbf",
			"amount": "0x3bb40760b029a49732fa"
		},
		"01d40dd4ac3a2dd4c5cc4ef36eabc7cc5d2d0593": {
			"balance": "0x2326e0c6ae4e5bc00000",
			"amount": "0x8c9b831ab9396f000000",
			"proxiedList": {
				"0x95413ce0a322fee7da9175a88a8603e5a15dec55": "0x1fc3842bd1f071c000000"
			},
			"candidate": true
		},
		"07c903846cc6c8ed62838771939a566c63296e1d": {
			"balance": "0x26f6a8f4e63803000000",
			"amount": "0x9bdaa3d398e00c000000"
		},
		"0832c3b801319b62ab1d3535615d1fe9afc3397a": {
			"balance": "0xa2f004a7411fe968dd97",
			"amount": "0x0",
			"delegate": "0x28bc0129d047fa5a37658"
		},
		"0833a5e02e12532ec64c2ac1ebf3a1014efea2c5": {
			"balance": "0x21753279401f66c00000",
			"amount": "0x85d4c9e5007d9b000000"
		},
		"096293c7fa66fc264c5feecfd4d343063f314992": {
			"balance": "0x1b24513071a43532d000000",
			"amount": "0x0"
		},
		"0b3ab1e226d382285dad43deeacccf839de98261": {
			"balance": "0x7f0e10af47c1c7000000",
			"amount": "0x1fc3842bd1f071c000000"
		},
		"0cc0a3ad2d371d63f2c8127b650a51d0d5ab7a9b": {
			"balance": "0x10d76c03cdb04d6888000",
			"amount": "0x435db00f36c135a220000"
		},
		"0d80bdb335b44b4090fbddd112f451ad32561e1e": {
			"balance": "0xa200618ec7395800000",
			"amount": "0x28801863b1ce56000000"
		},
		"0db8de123a49e81eb056b78151ee6729bbb865ca": {
			"balance": "0x27cf801b9d4f7d800000",
			"amount": "0x9f3e006e753df6000000"
		},
		"0e1129ea2e92b0a049192c2c63cbc201f7b2e634": {
			"balance": "0x29812e690b7e72800000",
			"amount": "0xa604b9a42df9ca000000"
		},
		"10ff593f75a5f53ea6270efda9fbb51d7b0aeae8": {
			"balance": "0x27cf801b9d4f7d800000",
			"amount": "0x9f3e006e753df6000000"
		},
		"12e644905c8525ab704a983bf7a9c8bfbdefcb1f": {
			"balance": "0x1eeaad051ad8f7400000",
			"amount": "0x7baab4146b63dd000000",
			"proxiedList": {
				"0x0832c3b801319b62ab1d3535615d1fe9afc3397a": "0x28bc0129d047fa5a37658"
			},
			"candidate": true
		},
		"133d604a2a138f04db8fb7d1f57fd739ad4b08aa": {
			"balance": "0xbd30250c0cbb9969e800",
			"amount": "0x0",
			"delegate": "0x2f4c0943032ee65a7a000"
		},
		"175624f126addc745f912ea95e7f6a02d501b1a7": {
			"balance": "0x1e7e4171bf4d3a000000",
			"amount": "0x79f905c6fd34e8000000",
			"proxiedList": {
				"0x7e4f269cc54e2dce11882176b386e80cdd8bc01b": "0x3a96ea715a63c5a7490a0"
			},
			"candidate": true
		},
		"1af7432241f03a83df616b33c00ad9d8ee6c59be": {
			"balance": "0x2a2baf5104ed6f36b333",
			"amount": "0xa8aebd4413b5bcdacccc"
		},
		"1caf686131a75f705e2fe211dfbc67840c975861": {
			"balance": "0x1e7e4171bf4d3a000000",
			"amount": "0x79f905c6fd34e8000000",
			"proxiedList": {
				"0x961367bf7177e794e932734b246c74a31a6c51d7": "0x1fc384dd74acb628db0c0"
			},
			"candidate": true
		},
		"2909db9e1668424806475aa4bba543431e785228": {
			"balance": "0x27cf801b9d4f7d800000",
			"amount": "0x9f3e006e753df6000000"
		},
		"29c6aaa742c98000f56b022bfdc68ac18070e0ab": {
			"balance": "0x99d62635bade22eb000",
			"amount": "0x0",
			"delegate": "0x2675898d6eb788bac000"
		},
		"2b12d852338ef5b1458b45bd49b0ef378c4d51b5": {
			"balance": "0xa05b2fe7d47a96a2079f",
			"amount": "0x0",
			"delegate": "0x2816cbf9f51ea5a881e7b"
		},
		"2ddd01fe6c2ebb94d43cc9d12de84e8eb2ecb6ea": {
			"balance": "0x1b7a1fe6e8bd3552ecc9",
			"amount": "0x6de87f9ba2f4d54bb323"
		},
		"2f9a2b62a8a7030d71814101ed06d9723bdbea2e": {
			"balance": "0x914e8389b0eddcbbb98",
			"amount": "0x2453a0e26c3b772eee60"
		},
		"328217bb3b8df19b3dab4f3ac7c9c8d9243f1f3f": {
			"balance": "0x1b24513071a43532d000000",
			"amount": "0x0"
		},
		"34f7b242142597e352096bc603e87c27f94dac1a": {
			"balance": "0x440708168f97cea9e382",
			"amount": "0x1101c205a3e5f3aa78e06"
		},
		"3abdb31086de7a75f93c9ee4b3f5626afcf8ab67": {
			"balance": "0xb352eb6a83ce7bb5c81",
			"amount": "0x2cd4badaa0f39eed7203"
		},
		"41e0a683ee564f73f1ca240d35e30a71a3848c14": {
			"balance": "0x4009230878222d000000",
			"amount": "0x100248c21e088b4000000"
		},
		"44461dfa999f908460898820664a13c2b0230fd2": {
			"balance": "0x834a4470db372b800000",
			"amount": "0x20d2911c36cdcae000000"
		},
		"44e4c3e17b5de1f7b9d40b2a6342c4551dba2e89": {
			"balance": "0x336a1bbdfa4c37b7f800",
			"amount": "0x0",
			"delegate": "0xcda86ef7e930dedfe000"
		},
		"4cacc12227f6594c7b0d4b741cf751ddffeb86fb": {
			"balance": "0x8a576baca8b87433c00",
			"amount": "0x2295daeb2a2e1d0cf000"
		},
		"4e8d522e7d95690bb44f7c589067cf06ba3b4e34": {
			"balance": "0x1329cf6b3798180929e3",
			"amount": "0x0",
			"delegate": "0x4ca73dacde606024a78c"
		},
		"5078ffa5b2ea521a77c5069dd77fa88c3526a0d0": {
			"balance": "0x159f40d4afcf20b40000",
			"amount": "0x0",
			"delegate": "0x567d0352bf3c82d00000"
		},
		"566c5cecd64f0fe75d6bdfaf6bb8a20f16cb7684": {
			"balance": "0x6bd174842840770da000",
			"amount": "0x1af45d210a101dc368000"
		},
		"58599d138f246ad9a2bc2d44dc145006aa6107f9": {
			"balance": "0x9db39b9d11c68d26800",
			"amount": "0x276ce6e74471a349a000"
		},
		"5a435816212f8e8932c8b6497afd2be2f5b7958a": {
			"balance": "0x2a5d17dbd08b669f5000",
			"amount": "0x0",
			"delegate": "0xa9745f6f422d9a7d4000"
		},
		"5a8e69a43e22f882de1b6467f0245bc06d498046": {
			"balance": "0x12a82d9b9b9fabcef8000",
			"amount": "0x4aa0b66e6e7eaf3be0000"
		},
		"614036008da1b1dac87ba923c6a883a1b80fb136": {
			"balance": "0x261dd1ce2f2088800000",
			"amount": "0x98774738bc8222000000"
		},
		"6353ca9b52af9da24d7e43d16fbe86dedb89eac7": {
			"balance": "0x2abcb750648b86f86000",
			"amount": "0x0",
			"delegate": "0xaaf2dd41922e1be18000"
		},
		"635773ee3bab9c168a4ae3292f5d9b14ae10eaa0": {
			"balance": "0xb1e07dc231427d000000",
			"amount": "0x2c781f708c509f4000000"
		},
		"653da4278ec15cdcd14d985a5989468c2491009c": {
			"balance": "0x9232527396140ea8000",
			"amount": "0x248c949ce58503aa0000"
		},
		"6645eeff68cf9a42846b9e7608a6efaf2308b528": {
			"balance": "0x26f6a8f4e63803000000",
			"amount": "0x9bdaa3d398e00c000000"
		},
		"6975d3c17d8809e9c7d17769c6dbd327893f84dd": {
			"balance": "0x261dd1ce2f2088800000",
			"amount": "0x98774738bc8222000000"
		},
		"6997c059fce895f155d4a3024507558659505cf3": {
			"balance": "0xaa3da1a0c39106c00000",
			"amount": "0x2a8f686830e441b000000"
		},
		"6a3c07fa2b7ed2d0a7d0151d123e0f0cdffa3dbb": {
			"balance": "0x121c35cdfb43b8dc9000",
			"amount": "0x0",
			"delegate": "0x4870d737ed0ee3724000"
		},
		"6dd5f29de57746aa0066e72b670e308340bc524d": {
			"balance": "0x4c148af34355e5480000",
			"amount": "0x0",
			"delegate": "0x130522bcd0d5795200000"
		},
		"6df9a65a7385aa70736c7508012060c0bd70adac": {
			"balance": "0x33ab4439a09830800000",
			"amount": "0xcead10e68260c2000000"
		},
		"6e6d80bd14a9a4c79a37fda65277045a0a8815a7": {
			"balance": "0x7f0e10af47c1c7000000",
			"amount": "0x1fc3842bd1f071c000000"
		},
		"715a0736212f16a152c7f921af6ffad117af3229": {
			"balance": "0x33ab4439a09830800000",
			"amount": "0xcead10e68260c2000000"
		},
		"723c1b86c78a04c4f125df4573acb0625bfc69a5": {
			"balance": "0x1eeaad051ad8f7400000",
			"amount": "0x7baab4146b63dd000000",
			"proxiedList": {
				"0x44e4c3e17b5de1f7b9d40b2a6342c4551dba2e89": "0xcda86ef7e930dedfe000",
				"0x6dd5f29de57746aa0066e72b670e308340bc524d": "0x130522bcd0d5795200000",
				"0xc22eff753847e407346e257a82f058499eb066cd": "0x10ab616b98d4330130000"
			},
			"candidate": true
		},
		"7491cf9eb06c7f12ddeeee6e3524bc83898c5343": {
			"balance": "0x7bbe3815580741a00000",
			"amount": "0x1eef8e055601d06800000"
		},
		"75b9eb7e604b110e47e1196c22c32886fa2a249c": {
			"balance": "0x1c07eedab7e8390c0000",
			"amount": "0x0",
			"delegate": "0x701fbb6adfa0e4300000"
		},
		"75e4dbb9f429d6a3475e4f84a87ed8e4be748f6b": {
			"balance": "0xd3c0612608ce3a28000",
			"amount": "0x0",
			"delegate": "0x34f0184982338e8a0000"
		},
		"7e4f269cc54e2dce11882176b386e80cdd8bc01b": {
			"balance": "0xea5ba9c5698f169d2428",
			"amount": "0x0",
			"delegate": "0x3a96ea715a63c5a7490a0"
		},
		"7ea3fa5bbb62e3f430019a5b46f74bf4ab08f49e": {
			"balance": "0x4ed1afa4ba7702ac4eaa",
			"amount": "0x13b46be92e9dc0ab13aa7"
		},
		"8052e254f37a76d4cecd402cd48955aefd78091c": {
			"balance": "0x2763148841c3c0400000",
			"amount": "0x9d8c5221070f01000000"
		},
		"819e020e3f1eef1333b4c7a144013cce623b5a92": {
			"balance": "0xb2a25b89a92e5f72480c",
			"amount": "0x2ca896e26a4b97dc92030"
		},
		"8cecac2453a8d0257407878ad4e8eee5870d5200": {
			"balance": "0x2fa261b8c7b23c8aa000",
			"amount": "0x0",
			"delegate": "0xbe8986e31ec8f22a8000"
		},
		"8fd54ed3637f59eefd0ba9d213fe30efe9d89ff3": {
			"balance": "0x21e19e0c9bab24000000",
			"amount": "0x878678326eac90000000"
		},
		"905a374898130c7f974a1fcdc0b53a1d324d16ce": {
			"balance": "0xa55740b8684d6800000",
			"amount": "0x2955d02e1a135a000000"
		},
		"913851b8d8afc4f6ac89241d3979e23424674d1f": {
			"balance": "0xa9756df479502f4b300",
			"amount": "0x2a5d5b7d1e540bd2cc00"
		},
		"9351e3962a708c92b78bfe640d224f33055e90ba": {
			"balance": "0x1e11d5de63c17cc00000",
			"amount": "0x784757798f05f3000000"
		},
		"95413ce0a322fee7da9175a88a8603e5a15dec55": {
			"balance": "0x7f0e10af47c1c7000000",
			"amount": "0x0",
			"delegate": "0x1fc3842bd1f071c000000"
		},
		"961367bf7177e794e932734b246c74a31a6c51d7": {
			"balance": "0x7f0e1375d2b2d8a36c30",
			"amount": "0x0",
			"delegate": "0x1fc384dd74acb628db0c0"
		},
		"967a24daa9394faf0f2a1e1da8f11acfbec8e2a4": {
			"balance": "0x2d82fee9c4274d79c6c0",
			"amount": "0xb60bfba7109d35e71b00"
		},
		"97b2478672d246780e36deb632ec5dbb0fc231bb": {
			"balance": "0x90d9db6d210b845a000",
			"amount": "0x243676db4842e1168000"
		},
		"981d9cd05117a9d84437958863374a9bc78ac988": {
			"balance": "0x72fa7d9e000a41bccf6",
			"amount": "0x0",
			"delegate": "0x1cbe9f678002906f33d4"
		},
		"99c3dc791f29e98255c197e7fe96f0933723db3f": {
			"balance": "0x224e099ff736e1400000",
			"amount": "0x8938267fdcdb85000000",
			"proxiedList": {
				"0x2b12d852338ef5b1458b45bd49b0ef378c4d51b5": "0x2816cbf9f51ea5a881e7b"
			},
			"candidate": true
		},
		"9a4eb75fc8db5680497ac33fd689b536334292b0": {
			"balance": "0x69e244c42fc8a9400000",
			"amount": "0x1a7891310bf22a5000000"
		},
		"9c10acb1ceb87ccd44a04000964ca192701dd392": {
			"balance": "0x57fa768bc71619c0bf80",
			"amount": "0x0",
			"delegate": "0x15fe9da2f1c586702fdfe"
		},
		"a8be4f3ee1cd772d22927fba3b1fe16e9c5ac898": {
			"balance": "0x2108c6e5e493a9800000",
			"amount": "0x84231b97924ea6000000",
			"proxiedList": {
				"0x6353ca9b52af9da24d7e43d16fbe86dedb89eac7": "0xaaf2dd41922e1be18000",
				"0xbecabc3fed76ca7a551d4c372c20318b7457878c": "0xdcb4c73d1742e456dd5f",
				"0xf5005b496dff7b1ba3ca06294f8f146c9afbe09d": "0xba58e545582d46000000"
			},
			"candidate": true
		},
		"a8e6fc44f082bebbde0ddf34597b40653e293e19": {
			"balance": "0x8de0c5d4cbdca6c00000",
			"amount": "0x2378317532f729b000000"
		},
		"a970d175e16b74c868b9a25657da9b2385dd1320": {
			"balance": "0xbf25435ac5218037ffc1",
			"amount": "0x0",
			"delegate": "0x2fc950d6b148600dfff00"
		},
		"ac7ab60e2f28b137be99a9f960f0d01af01faad5": {
			"balance": "0x458eacde2f92fdd8800",
			"amount": "0x0",
			"delegate": "0x1163ab378be4bf762000"
		},
		"b0534db09a8ad1fd6ee2e3ad6889a26799926ad8": {
			"balance": "0xee61264426410d1d5c2",
			"amount": "0x0",
			"delegate": "0x3b984991099043475706"
		},
		"b05acbd235d8410eb8749702b669191d5d572bc8": {
			"balance": "0x7f0e1375c081e52ec000",
			"amount": "0x1fc384dd7020794bb0000"
		},
		"b25b83408b03667a277161f0260dfed78a871ca4": {
			"balance": "0x29ed99fc670a2fc00000",
			"amount": "0xa7b667f19c28bf000000"
		},
		"b38a0efd97eb49fa5085634de85594fe4e0a69bd": {
			"balance": "0x34841b6057afab000000",
			"amount": "0xd2106d815ebeac000000"
		},
		"becabc3fed76ca7a551d4c372c20318b7457878c": {
			"balance": "0x372d31cf45d0b915b758",
			"amount": "0x0",
			"delegate": "0xdcb4c73d1742e456dd5f"
		},
		"c1bad2e147c83b13d1428ce118dc836fbcc75c26": {
			"balance": "0x68be07c5df49fb48cd7c",
			"amount": "0x0",
			"delegate": "0x1a2f81f177d27ed2335ee"
		},
		"c22eff753847e407346e257a82f058499eb066cd": {
			"balance": "0x42ad85ae6350cc04c000",
			"amount": "0x0",
			"delegate": "0x10ab616b98d4330130000"
		},
		"c4483b02ce41185de5bd4fd25f72050d3109db88": {
			"balance": "0x31f999a1f07b4f652384",
			"amount": "0xc7e66687c1ed3d948e0f"
		},
		"c46f7498d76ea5d15173edf0e6a9ba59c68f666a": {
			"balance": "0xa968163f0a57b4000000",
			"amount": "0x2a5a058fc295ed0000000"
		},
		"c56c8b2ffad06dc2c07a681e4af9f856400a70e4": {
			"balance": "0x46a66c0fdad4fc680000",
			"amount": "0x11a99b03f6b53f1a00000"
		},
		"c72f78ab2334a0a8acdbdc53c71186b07ee7580d": {
			"balance": "0xf6aa91f09e2d002b6ab",
			"amount": "0x0",
			"delegate": "0x3daaa47c278b400adaaa"
		},
		"c7be103eef93ec9946d6bca67894ca80042895ee": {
			"balance": "0x1b822a0671cfef2400000",
			"amount": "0x6e08a819c73fbc9000000"
		},
		"c91b4740013b79320503c0cf1708b8e4fb05f9a5": {
			"balance": "0x3b816e0e270adf6ae454",
			"amount": "0xee05b8389c2b7dab9150"
		},
		"ce8766bd3a66b1cc2b948482259830b5007591eb": {
			"balance": "0x3a9bb437963c24495800",
			"amount": "0x0"
		},
		"d06b6c1299c04b71febc1e6951458ebdd5a04f7d": {
			"balance": "0x5ba622902f52c6800000",
			"amount": "0x16e988a40bd4b1a000000"
		},
		"d0ab0c3a8391d6545f27ef785103647b673343b4": {
			"balance": "0x25b1663ad394cb400000",
			"amount": "0x96c598eb4e532d000000",
			"proxiedList": {
				"0xa970d175e16b74c868b9a25657da9b2385dd1320": "0x2fc950d6b148600dfff00"
			},
			"candidate": true
		},
		"d6fb7034433e11663500038c124d7c3abdd9d63c": {
			"balance": "0x3f870857a3e0e3800000",
			"amount": "0xfe1c215e8f838e000000"
		},
		"d71b10611d39d5b4c2b2aca0f9c48d0fd60941b8": {
			"balance": "0x333ed8a6450c73400000",
			"amount": "0xccfb62991431cd000000"
		},
		"d88a986de29c97a124d7169fa5cbeff73e28bdd2": {
			"balance": "0x1e7e4171bf4d3a000000",
			"amount": "0x79f905c6fd34e8000000",
			"proxiedList": {
				"0x9c10acb1ceb87ccd44a04000964ca192701dd392": "0x15fe9da2f1c586702fdfe",
				"0xc1bad2e147c83b13d1428ce118dc836fbcc75c26": "0x1a2f81f177d27ed2335ee"
			},
			"candidate": true
		},
		"db76d6ecce3b1f8d764741d9cdb3d4f294e1dce2": {
			"balance": "0x6fbc6bfc153e45a60847",
			"amount": "0x1bef1aff054f916982119"
		},
		"dbee5cdbb6dd622b0b09ebe6d39e4902a54de8e4": {
			"balance": "0x179f1820b3023be800000",
			"amount": "0x5e7c6082cc08efa000000"
		},
		"dccfb63166a623b5d9cb9481565101b667617cf9": {
			"balance": "0xba79951f49b54c612036",
			"amount": "0x2e9e6547d26d5318480d7"
		},
		"e2e476156b69af23a12750426734ded8bae7d01e": {
			"balance": "0x26f6a8f4e63803000000",
			"amount": "0x9bdaa3d398e00c000000"
		},
		"e4da810f56e27ba24364621da2b0c3b571355f65": {
			"balance": "0x6b665385c8f48580000",
			"amount": "0x0",
			"delegate": "0x1ad994e1723d21600000"
		},
		"e59dd79111a34ee11ce1686f1c3e12d8d1e63652": {
			"balance": "0x35c682feeffdd70768ee",
			"amount": "0xd71a0bfbbff75c1da3b6"
		},
		"e8a9d428342507c2b4f6ba442cd29bb8e48f6ec4": {
			"balance": "0x2d318553c04f18907e13",
			"amount": "0xb4c6154f013c6241f84b"
		},
		"ea80fd3fcc41441075fad44302548cd484a6c7af": {
			"balance": "0x1e11d5de63c17cc00000",
			"amount": "0x784757798f05f3000000",
			"proxiedList": {
				"0x5a435816212f8e8932c8b6497afd2be2f5b7958a": "0xa9745f6f422d9a7d4000",
				"0x75b9eb7e604b110e47e1196c22c32886fa2a249c": "0x701fbb6adfa0e4300000",
				"0x8cecac2453a8d0257407878ad4e8eee5870d5200": "0xbe8986e31ec8f22a8000"
			},
			"candidate": true
		},
		"ed0658b5b05b6fa365bed42746706f21c109447f": {
			"balance": "0x1c93c10c327d63e2915ea3d",
			"amount": "0x0"
		},
		"ef470c3a63343585651808b8187bba0e277bc3c8": {
			"balance": "0x1ec0668d3c90c00946b0",
			"amount": "0x7b019a34f24300251abf"
		},
		"f1d72486483e60bfd8e5cfbd9df22ad15238b46e": {
			"balance": "0x261fb28aa29bfa8d524a",
			"amount": "0x987eca2a8a6fea354927"
		},
		"f2e81e37c8f8710780b3cf834fc16aa3375699b6": {
			"balance": "0x261dd1ce2f2088800000",
			"amount": "0x98774738bc8222000000"
		},
		"f35d12756790c527f538e00de07c837a45e72732": {
			"balance": "0x32d26d12e980b6000000",
			"amount": "0xcb49b44ba602d8000000"
		},
		"f45ced1304c7dddd6a7012361f4a3d8f9ea03b7f": {
			"balance": "0x22ba753352c29e800000",
			"amount": "0x8ae9d4cd4b0a7a000000",
			"proxiedList": {
				"0x133d604a2a138f04db8fb7d1f57fd739ad4b08aa": "0x2f4c0943032ee65a7a000"
			},
			"candidate": true
		},
		"f5005b496dff7b1ba3ca06294f8f146c9afbe09d": {
			"balance": "0x2e963951560b51800000",
			"amount": "0x0",
			"delegate": "0xba58e545582d46000000"
		},
		"f6a94ecd6347a051709f7f96061c6a0265b59181": {
			"balance": "0x7a3259b70664d53e9e46",
			"amount": "0x1e8c966dc199354fa7918"
		},
		"f92f0b506feb77693c51507eeea62c74869ff0f0": {
			"balance": "0x1e11d5de63c17cc00000",
			"amount": "0x784757798f05f3000000",
			"proxiedList": {
				"0x29c6aaa742c98000f56b022bfdc68ac18070e0ab": "0x2675898d6eb788bac000",
				"0x4e8d522e7d95690bb44f7c589067cf06ba3b4e34": "0x4ca73dacde606024a78c",
				"0x5078ffa5b2ea521a77c5069dd77fa88c3526a0d0": "0x567d0352bf3c82d00000",
				"0x6a3c07fa2b7ed2d0a7d0151d123e0f0cdffa3dbb": "0x4870d737ed0ee3724000",
				"0x75e4dbb9f429d6a3475e4f84a87ed8e4be748f6b": "0x34f0184982338e8a0000",
				"0x981d9cd05117a9d84437958863374a9bc78ac988": "0x1cbe9f678002906f33d4",
				"0xac7ab60e2f28b137be99a9f960f0d01af01faad5": "0x1163ab378be4bf762000",
				"0xb0534db09a8ad1fd6ee2e3ad6889a26799926ad8": "0x3b984991099043475706",
				"0xc72f78ab2334a0a8acdbdc53c71186b07ee7580d": "0x3daaa47c278b400adaaa",
				"0xe4da810f56e27ba24364621da2b0c3b571355f65": "0x1ad994e1723d21600000"
			},
			"candidate": true
		},
		"fd9cec784dde0d65a6517d0f24b2344bde1b4aac": {
			"balance": "0x5ac423da65ab3ecef8e4",
			"amount": "0x16b108f6996acfb3be38f"
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
				"oosBlock": 11800000,
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
        "coinbase": "0x0000000000000000000000000000000000000000",
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
