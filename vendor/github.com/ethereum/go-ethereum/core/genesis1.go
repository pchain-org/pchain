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
	"coinbase": "0x0000000000000000000000000000000000000000",
	"alloc": {
		"0168647b3d5981d9a0711a0339052a016fb6e1b7": {
			"balance": "0x0",
			"amount": "0x4aa10938dc340dbcffb9"
		},
		"01d40dd4ac3a2dd4c5cc4ef36eabc7cc5d2d0593": {
			"balance": "0x0",
			"amount": "0xafc263e16787cac00000",
			"proxiedList": {
				"0x95413ce0a322fee7da9175a88a8603e5a15dec55": "0x27b46536c66c8e3000000"
			},
			"candidate": true
		},
		"07c903846cc6c8ed62838771939a566c63296e1d": {
			"balance": "0x0",
			"amount": "0xc2d14cc87f180f000000"
		},
		"0832c3b801319b62ab1d3535615d1fe9afc3397a": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x32eb01744459f8f0c53ef"
		},
		"0833a5e02e12532ec64c2ac1ebf3a1014efea2c5": {
			"balance": "0x0",
			"amount": "0xa749fc5e409d01c00000"
		},
		"096293c7fa66fc264c5feecfd4d343063f314992": {
			"balance": "0x1b24513071a43532d000000",
			"amount": "0x0"
		},
		"0b3ab1e226d382285dad43deeacccf839de98261": {
			"balance": "0x0",
			"amount": "0x27b46536c66c8e3000000"
		},
		"0cc0a3ad2d371d63f2c8127b650a51d0d5ab7a9b": {
			"balance": "0x0",
			"amount": "0x54351c130471830aa8000"
		},
		"0d80bdb335b44b4090fbddd112f451ad32561e1e": {
			"balance": "0x0",
			"amount": "0x32a01e7c9e41eb800000"
		},
		"0db8de123a49e81eb056b78151ee6729bbb865ca": {
			"balance": "0x0",
			"amount": "0xc70d808a128d73800000"
		},
		"0e1129ea2e92b0a049192c2c63cbc201f7b2e634": {
			"balance": "0x0",
			"amount": "0xcf85e80d39783c800000"
		},
		"10ff593f75a5f53ea6270efda9fbb51d7b0aeae8": {
			"balance": "0x0",
			"amount": "0xc70d808a128d73800000"
		},
		"12e644905c8525ab704a983bf7a9c8bfbdefcb1f": {
			"balance": "0x0",
			"amount": "0x9a956119863cd4400000",
			"proxiedList": {
				"0x0832c3b801319b62ab1d3535615d1fe9afc3397a": "0x32eb01744459f8f0c53ef"
			},
			"candidate": true
		},
		"133d604a2a138f04db8fb7d1f57fd739ad4b08aa": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x3b1f0b93c3fa9ff118800"
		},
		"175624f126addc745f912ea95e7f6a02d501b1a7": {
			"balance": "0x0",
			"amount": "0x98774738bc8222000000",
			"proxiedList": {
				"0x7e4f269cc54e2dce11882176b386e80cdd8bc01b": "0x493ca50db0fcb7111b4c8"
			},
			"candidate": true
		},
		"1af7432241f03a83df616b33c00ad9d8ee6c59be": {
			"balance": "0x0",
			"amount": "0xd2da6c9518a32c117fff"
		},
		"1caf686131a75f705e2fe211dfbc67840c975861": {
			"balance": "0x0",
			"amount": "0x98774738bc8222000000",
			"proxiedList": {
				"0x961367bf7177e794e932734b246c74a31a6c51d7": "0x27b46614d1d7e3b311cf0"
			},
			"candidate": true
		},
		"2909db9e1668424806475aa4bba543431e785228": {
			"balance": "0x0",
			"amount": "0xc70d808a128d73800000"
		},
		"29c6aaa742c98000f56b022bfdc68ac18070e0ab": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x3012ebf0ca656ae97000"
		},
		"2b12d852338ef5b1458b45bd49b0ef378c4d51b5": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x321c7ef872664f12a261a"
		},
		"2ddd01fe6c2ebb94d43cc9d12de84e8eb2ecb6ea": {
			"balance": "0x0",
			"amount": "0x89629f828bb20a9e9fec"
		},
		"2f9a2b62a8a7030d71814101ed06d9723bdbea2e": {
			"balance": "0x0",
			"amount": "0x2d68891b074a54faa9f8"
		},
		"328217bb3b8df19b3dab4f3ac7c9c8d9243f1f3f": {
			"balance": "0x1b24513071a43532d000000",
			"amount": "0x0"
		},
		"34f7b242142597e352096bc603e87c27f94dac1a": {
			"balance": "0x0",
			"amount": "0x154232870cdf709517188"
		},
		"3abdb31086de7a75f93c9ee4b3f5626afcf8ab67": {
			"balance": "0x0",
			"amount": "0x3809e991493086a8ce84"
		},
		"41e0a683ee564f73f1ca240d35e30a71a3848c14": {
			"balance": "0x0",
			"amount": "0x1402daf2a58aae1000000"
		},
		"44461dfa999f908460898820664a13c2b0230fd2": {
			"balance": "0x0",
			"amount": "0x2907356344813d9800000"
		},
		"44e4c3e17b5de1f7b9d40b2a6342c4551dba2e89": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x101128ab5e37d1697d800"
		},
		"4cacc12227f6594c7b0d4b741cf751ddffeb86fb": {
			"balance": "0x0",
			"amount": "0x2b3b51a5f4b9a4502c00"
		},
		"4e8d522e7d95690bb44f7c589067cf06ba3b4e34": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x5fd10d1815f8782dd16f"
		},
		"5078ffa5b2ea521a77c5069dd77fa88c3526a0d0": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x6c1c44276f0ba3840000"
		},
		"566c5cecd64f0fe75d6bdfaf6bb8a20f16cb7684": {
			"balance": "0x0",
			"amount": "0x21b174694c94253442000"
		},
		"58599d138f246ad9a2bc2d44dc145006aa6107f9": {
			"balance": "0x0",
			"amount": "0x314820a1158e0c1c0800"
		},
		"5a435816212f8e8932c8b6497afd2be2f5b7958a": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0xd3d1774b12b9011c9000"
		},
		"5a8e69a43e22f882de1b6467f0245bc06d498046": {
			"balance": "0x0",
			"amount": "0x5d48e40a0a1e5b0ad8000"
		},
		"614036008da1b1dac87ba923c6a883a1b80fb136": {
			"balance": "0x0",
			"amount": "0xbe951906eba2aa800000"
		},
		"6353ca9b52af9da24d7e43d16fbe86dedb89eac7": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0xd5af9491f6b9a2d9e000"
		},
		"635773ee3bab9c168a4ae3292f5d9b14ae10eaa0": {
			"balance": "0x0",
			"amount": "0x3796274caf64c71000000"
		},
		"653da4278ec15cdcd14d985a5989468c2491009c": {
			"balance": "0x0",
			"amount": "0x2dafb9c41ee644948000"
		},
		"6645eeff68cf9a42846b9e7608a6efaf2308b528": {
			"balance": "0x0",
			"amount": "0xc2d14cc87f180f000000"
		},
		"6975d3c17d8809e9c7d17769c6dbd327893f84dd": {
			"balance": "0x0",
			"amount": "0xbe951906eba2aa800000"
		},
		"6997c059fce895f155d4a3024507558659505cf3": {
			"balance": "0x0",
			"amount": "0x353342823d1d521c00000"
		},
		"6a3c07fa2b7ed2d0a7d0151d123e0f0cdffa3dbb": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x5a8d0d05e8529c4ed000"
		},
		"6dd5f29de57746aa0066e72b670e308340bc524d": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x17c66b6c050ad7a680000"
		},
		"6df9a65a7385aa70736c7508012060c0bd70adac": {
			"balance": "0x0",
			"amount": "0x10258552022f8f2800000"
		},
		"6e6d80bd14a9a4c79a37fda65277045a0a8815a7": {
			"balance": "0x0",
			"amount": "0x27b46536c66c8e3000000"
		},
		"715a0736212f16a152c7f921af6ffad117af3229": {
			"balance": "0x0",
			"amount": "0x10258552022f8f2800000"
		},
		"723c1b86c78a04c4f125df4573acb0625bfc69a5": {
			"balance": "0x0",
			"amount": "0x9a956119863cd4400000",
			"proxiedList": {
				"0x44e4c3e17b5de1f7b9d40b2a6342c4551dba2e89": "0x101128ab5e37d1697d800",
				"0x6dd5f29de57746aa0066e72b670e308340bc524d": "0x17c66b6c050ad7a680000",
				"0xc22eff753847e407346e257a82f058499eb066cd": "0x14d639c67f093fc17c000"
			},
			"candidate": true
		},
		"7491cf9eb06c7f12ddeeee6e3524bc83898c5343": {
			"balance": "0x0",
			"amount": "0x26ab7186ab82448200000"
		},
		"75b9eb7e604b110e47e1196c22c32886fa2a249c": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x8c27aa4597891d3c0000"
		},
		"75e4dbb9f429d6a3475e4f84a87ed8e4be748f6b": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x422c1e5be2c0722c8000"
		},
		"7e4f269cc54e2dce11882176b386e80cdd8bc01b": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x493ca50db0fcb7111b4c8"
		},
		"7ea3fa5bbb62e3f430019a5b46f74bf4ab08f49e": {
			"balance": "0x0",
			"amount": "0x18a186e37a4530d5d8951"
		},
		"8052e254f37a76d4cecd402cd48955aefd78091c": {
			"balance": "0x0",
			"amount": "0xc4ef66a948d2c1400000"
		},
		"819e020e3f1eef1333b4c7a144013cce623b5a92": {
			"balance": "0x0",
			"amount": "0x37d2bc9b04de7dd3b683c"
		},
		"8cecac2453a8d0257407878ad4e8eee5870d5200": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0xee2be89be67b2eb52000"
		},
		"8fd54ed3637f59eefd0ba9d213fe30efe9d89ff3": {
			"balance": "0x0",
			"amount": "0xa968163f0a57b4000000"
		},
		"905a374898130c7f974a1fcdc0b53a1d324d16ce": {
			"balance": "0x0",
			"amount": "0x33ab4439a09830800000"
		},
		"913851b8d8afc4f6ac89241d3979e23424674d1f": {
			"balance": "0x0",
			"amount": "0x34f4b25c65e90ec77f00"
		},
		"9351e3962a708c92b78bfe640d224f33055e90ba": {
			"balance": "0x0",
			"amount": "0x96592d57f2c76fc00000"
		},
		"95413ce0a322fee7da9175a88a8603e5a15dec55": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x27b46536c66c8e3000000"
		},
		"961367bf7177e794e932734b246c74a31a6c51d7": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x27b46614d1d7e3b311cf0"
		},
		"967a24daa9394faf0f2a1e1da8f11acfbec8e2a4": {
			"balance": "0x0",
			"amount": "0xe38efa90d4c48360e1c0"
		},
		"97b2478672d246780e36deb632ec5dbb0fc231bb": {
			"balance": "0x0",
			"amount": "0x2d4414921a53995c2000"
		},
		"981d9cd05117a9d84437958863374a9bc78ac988": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x23ee47416003348b00ca"
		},
		"99c3dc791f29e98255c197e7fe96f0933723db3f": {
			"balance": "0x0",
			"amount": "0xab86301fd41266400000",
			"proxiedList": {
				"0x2b12d852338ef5b1458b45bd49b0ef378c4d51b5": "0x321c7ef872664f12a261a"
			},
			"candidate": true
		},
		"9a4eb75fc8db5680497ac33fd689b536334292b0": {
			"balance": "0x0",
			"amount": "0x2116b57d4eeeb4e400000"
		},
		"9c10acb1ceb87ccd44a04000964ca192701dd392": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x1b7e450bae36e80c3bd7e"
		},
		"a8be4f3ee1cd772d22927fba3b1fe16e9c5ac898": {
			"balance": "0x0",
			"amount": "0xa52be27d76e24f800000",
			"proxiedList": {
				"0x6353ca9b52af9da24d7e43d16fbe86dedb89eac7": "0xd5af9491f6b9a2d9e000",
				"0xbecabc3fed76ca7a551d4c372c20318b7457878c": "0x113e1f90c5d139d6c94b7",
				"0xf5005b496dff7b1ba3ca06294f8f146c9afbe09d": "0xe8ef1e96ae3897800000"
			},
			"candidate": true
		},
		"a8e6fc44f082bebbde0ddf34597b40653e293e19": {
			"balance": "0x0",
			"amount": "0x2c563dd27fb4f41c00000"
		},
		"a970d175e16b74c868b9a25657da9b2385dd1320": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x3bbba50c5d9a78117fec1"
		},
		"ac7ab60e2f28b137be99a9f960f0d01af01faad5": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x15bc96056eddef53a800"
		},
		"b0534db09a8ad1fd6ee2e3ad6889a26799926ad8": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x4a7e5bf54bf454192cc8"
		},
		"b05acbd235d8410eb8749702b669191d5d572bc8": {
			"balance": "0x0",
			"amount": "0x27b46614cc28979e9c000"
		},
		"b25b83408b03667a277161f0260dfed78a871ca4": {
			"balance": "0x0",
			"amount": "0xd1a401ee0332eec00000"
		},
		"b38a0efd97eb49fa5085634de85594fe4e0a69bd": {
			"balance": "0x0",
			"amount": "0x1069488e1b66e57000000"
		},
		"becabc3fed76ca7a551d4c372c20318b7457878c": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x113e1f90c5d139d6c94b7"
		},
		"c1bad2e147c83b13d1428ce118dc836fbcc75c26": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x20bb626dd5c71e86c036a"
		},
		"c22eff753847e407346e257a82f058499eb066cd": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x14d639c67f093fc17c000"
		},
		"c4483b02ce41185de5bd4fd25f72050d3109db88": {
			"balance": "0x0",
			"amount": "0x8cef5cfe8d8a9068e193"
		},
		"c46f7498d76ea5d15173edf0e6a9ba59c68f666a": {
			"balance": "0x0",
			"amount": "0x34f086f3b33b684000000"
		},
		"c56c8b2ffad06dc2c07a681e4af9f856400a70e4": {
			"balance": "0x0",
			"amount": "0x161401c4f4628ee080000"
		},
		"c72f78ab2334a0a8acdbdc53c71186b07ee7580d": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x4d154d9b316e100d9155"
		},
		"c7be103eef93ec9946d6bca67894ca80042895ee": {
			"balance": "0x0",
			"amount": "0x898ad220390fabb400000"
		},
		"c91b4740013b79320503c0cf1708b8e4fb05f9a5": {
			"balance": "0x0",
			"amount": "0x129872646c3365d1675a4"
		},
		"ce8766bd3a66b1cc2b948482259830b5007591eb": {
			"balance": "0x0",
			"amount": "0x3a9bb437963c24495800"
		},
		"d06b6c1299c04b71febc1e6951458ebdd5a04f7d": {
			"balance": "0x0",
			"amount": "0x1ca3eacd0ec9de0800000"
		},
		"d0ab0c3a8391d6545f27ef785103647b673343b4": {
			"balance": "0x0",
			"amount": "0xbc76ff2621e7f8400000",
			"proxiedList": {
				"0xa970d175e16b74c868b9a25657da9b2385dd1320": "0x3bbba50c5d9a78117fec1"
			},
			"candidate": true
		},
		"d6fb7034433e11663500038c124d7c3abdd9d63c": {
			"balance": "0x0",
			"amount": "0x13da329b6336471800000"
		},
		"d71b10611d39d5b4c2b2aca0f9c48d0fd60941b8": {
			"balance": "0x0",
			"amount": "0x1003a3b3f593e40400000"
		},
		"d88a986de29c97a124d7169fa5cbeff73e28bdd2": {
			"balance": "0x0",
			"amount": "0x98774738bc8222000000",
			"proxiedList": {
				"0x9c10acb1ceb87ccd44a04000964ca192701dd392": "0x1b7e450bae36e80c3bd7e",
				"0xc1bad2e147c83b13d1428ce118dc836fbcc75c26": "0x20bb626dd5c71e86c036a"
			},
			"candidate": true
		},
		"db76d6ecce3b1f8d764741d9cdb3d4f294e1dce2": {
			"balance": "0x0",
			"amount": "0x22eae1bec6a375c3e2960"
		},
		"dbee5cdbb6dd622b0b09ebe6d39e4902a54de8e4": {
			"balance": "0x0",
			"amount": "0x761b78a37f0b2b8800000"
		},
		"dccfb63166a623b5d9cb9481565101b667617cf9": {
			"balance": "0x0",
			"amount": "0x3a45fe99c708a7de5a10d"
		},
		"e2e476156b69af23a12750426734ded8bae7d01e": {
			"balance": "0x0",
			"amount": "0xc2d14cc87f180f000000"
		},
		"e4da810f56e27ba24364621da2b0c3b571355f65": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0x218ffa19cecc69b80000"
		},
		"e59dd79111a34ee11ce1686f1c3e12d8d1e63652": {
			"balance": "0x0",
			"amount": "0x10ce08efaaff533250ca4"
		},
		"e8a9d428342507c2b4f6ba442cd29bb8e48f6ec4": {
			"balance": "0x0",
			"amount": "0xe1f79aa2c18b7ad2765e"
		},
		"ea271dda6fed2116e1a0dbc8d0e9bae00b16fa7a": {
			"balance": "0x1ca133a2c467c7b27ebea3d",
			"amount": "0x0"
		},
		"ea80fd3fcc41441075fad44302548cd484a6c7af": {
			"balance": "0x0",
			"amount": "0x96592d57f2c76fc00000",
			"proxiedList": {
				"0x5a435816212f8e8932c8b6497afd2be2f5b7958a": "0xd3d1774b12b9011c9000",
				"0x75b9eb7e604b110e47e1196c22c32886fa2a249c": "0x8c27aa4597891d3c0000",
				"0x8cecac2453a8d0257407878ad4e8eee5870d5200": "0xee2be89be67b2eb52000"
			},
			"candidate": true
		},
		"ef470c3a63343585651808b8187bba0e277bc3c8": {
			"balance": "0x0",
			"amount": "0x99c200c22ed3c02e616f"
		},
		"f1d72486483e60bfd8e5cfbd9df22ad15238b46e": {
			"balance": "0x0",
			"amount": "0xbe9e7cb52d0be4c29b71"
		},
		"f2e81e37c8f8710780b3cf834fc16aa3375699b6": {
			"balance": "0x0",
			"amount": "0xbe951906eba2aa800000"
		},
		"f35d12756790c527f538e00de07c837a45e72732": {
			"balance": "0x0",
			"amount": "0xfe1c215e8f838e000000"
		},
		"f45ced1304c7dddd6a7012361f4a3d8f9ea03b7f": {
			"balance": "0x0",
			"amount": "0xada44a009dcd18800000",
			"proxiedList": {
				"0x133d604a2a138f04db8fb7d1f57fd739ad4b08aa": "0x3b1f0b93c3fa9ff118800"
			},
			"candidate": true
		},
		"f5005b496dff7b1ba3ca06294f8f146c9afbe09d": {
			"balance": "0x0",
			"amount": "0x0",
			"delegate": "0xe8ef1e96ae3897800000"
		},
		"f6a94ecd6347a051709f7f96061c6a0265b59181": {
			"balance": "0x0",
			"amount": "0x1f8c2fa9f9e9927f3e75e"
		},
		"f92f0b506feb77693c51507eeea62c74869ff0f0": {
			"balance": "0x0",
			"amount": "0x96592d57f2c76fc00000",
			"proxiedList": {
				"0x29c6aaa742c98000f56b022bfdc68ac18070e0ab": "0x3012ebf0ca656ae97000",
				"0x4e8d522e7d95690bb44f7c589067cf06ba3b4e34": "0x5fd10d1815f8782dd16f",
				"0x5078ffa5b2ea521a77c5069dd77fa88c3526a0d0": "0x6c1c44276f0ba3840000",
				"0x6a3c07fa2b7ed2d0a7d0151d123e0f0cdffa3dbb": "0x5a8d0d05e8529c4ed000",
				"0x75e4dbb9f429d6a3475e4f84a87ed8e4be748f6b": "0x422c1e5be2c0722c8000",
				"0x981d9cd05117a9d84437958863374a9bc78ac988": "0x23ee47416003348b00ca",
				"0xac7ab60e2f28b137be99a9f960f0d01af01faad5": "0x15bc96056eddef53a800",
				"0xb0534db09a8ad1fd6ee2e3ad6889a26799926ad8": "0x4a7e5bf54bf454192cc8",
				"0xc72f78ab2334a0a8acdbdc53c71186b07ee7580d": "0x4d154d9b316e100d9155",
				"0xe4da810f56e27ba24364621da2b0c3b571355f65": "0x218ffa19cecc69b80000"
			},
			"candidate": true
		},
		"fd9cec784dde0d65a6517d0f24b2344bde1b4aac": {
			"balance": "0x0",
			"amount": "0x1c5d4b343fc583a0adc73"
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
