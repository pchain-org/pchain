A full wiki with everything you need to know about Pchain is here: [Pchain Wiki](https://pliangroup.gitbook.io/plian/).

[Chinese (中文) Wiki](https://pchaindocscn.readthedocs.io/)

## Pchain

[中文 README](https://github.com/pchain-org/pchain/wiki/README_CN)

Official golang implementation of the Pchain protocol.

To get familiar with pchain, you could read about [Overview](https://pliangroup.gitbook.io/plian/)

## Installation Instructions for Ubuntu(Official recommendation 16.04)

Installing from PPA, or [install from release](https://pliangroup.gitbook.io/plian/using-the-blockchain/validator-node-guides/setting-up-a-validator-node)(If you want to become a Pchain validator, we recommend you to install from release)

```
sudo apt update
sudo apt upgrade
sudo apt install software-properties-common
sudo add-apt-repository -y ppa:pchainorg/pchain
sudo apt update
sudo apt install pchain
```

You should now be able to check the different options and commands with 'pchain --help'.

Upgrade the latest version of 'pchain'

```
sudo apt update
sudo apt dist-upgrade
```

## Installation Instructions for Mac OS X

You can use our Homebrew tap to install pchain. If you don't have Homebrew,[install it first](https://brew.sh/)

Then run the following commands to add the tap and install `pchain`:

```shell
brew tap pchain-org/pchain
brew install pchain
```

Upgrade the latest version of 'pchain'

```shell
brew upgrade pchain
```

You should now be able to check the different options and commands with 'pchain --help'.

After installing, run `pchain account new` to create an account on your node.

## Building the source

If you would like to build from the source:

Building pchain requires both a Go (version 1.10 or later) and a C compiler.
You can install them using your favourite package manager.
Once the dependencies are installed, run

```shell
git clone -b pre_mainnet https://github.com/pchain-org/pchain.git
cd pchain
make pchain
```

After run the command above,you will find pchain client in directory ./bin

```shell
./bin/pchain --help

```

If you want to build and run pchain under Windows(Not recommended), you could refer to [Build and start pchain](https://pliangroup.gitbook.io/plian/using-the-blockchain/validator-node-guides/alternate-installation-instructions/installation-instructions-for-windows-10)

If you want to know more about our Command Line Options,please consult our
[CLI Wiki page](https://pliangroup.gitbook.io/plian/using-the-blockchain/advanced-features/command-line-options)

### Please Check Your Pchain Client Version (Latest:1.6.02)

```
$ pchain version
//1.6.02
```

### Sync and run testnet

If you want to test your smart contract, it's better to deploy it on testnet first.
You can follow [this guide](https://pliangroup.gitbook.io/plian/using-the-blockchain/advanced-features/how-to-sync-and-run-testnet) to run pchain's testnet.

You can [Get free tPI](https://testnet.plian.org/faucet.html) from our testnet.

### Full Node On The Pchain Main Network

```
$ pchain
```

You can set your own data directory by '--datadir yourOwnDirectory'
If you want to open RPC,just add the options '--rpc' and '--rpcapi "db,eth,net,web3,admin,tdm,miner,personal,chain,txpool" '. More details,please read [How To Interact With Pchain](https://pliangroup.gitbook.io/plian/using-the-blockchain/advanced-features/json-rpc)

you can use 'nohup' and add '&' at the end of command above, Pchain client will run in the background

```
$ nohup pchain &
```

| Platform | Default Datadir Directory |
| :------- | :------------------------ |
| Linux    | ~/.pchain                 |
| Mac      | ~/Library/Pchain          |
| Windows  | %APPDATA%\Pchain          |

### Docker quick start

One of the quickest ways to get Pchain up and running on your machine is by using Docker:

```
docker run -d --name pchain-node -v ~/pchain/.pchain:/.pchain \
           -p 6969:6969 -p 30308:30308 \
           plian/pchain --datadir=/.pchain
```

This will start pchain just as the above command does. It will also create a persistent volume in your home directory for saving your blockchain as well as map the default ports.

Do not forget `--rpcaddr=0.0.0.0 --rpc --rpcapi=eth,web3,admin,tdm,miner,personal,chain,txpool,del`, if you want to access RPC from other containers and/or hosts. By default, pchain binds to the local interface and RPC endpoints is not accessible from the outside.

### How To Interact With Pchain(RPC OR IPC CONSOLE)

Please read the wiki [How to Use the IPC Console](https://pliangroup.gitbook.io/plian/using-the-blockchain/pchain-console)

Please Read The Wiki [How To Use the JSON RPC](https://pliangroup.gitbook.io/plian/using-the-blockchain/advanced-features/json-rpc)

### How To Become A Validator

For a full guide on how to set up and become a validator node on the Pchain Network, see the [Validator Node Guides](https://pliangroup.gitbook.io/plian/using-the-blockchain/validator-node-guides)

### How To Delegate Your Stake

To learn how to delegate your PI to validators to receive rewards, read [How To Delegate or Stake](https://pliangroup.gitbook.io/plian/using-the-blockchain/delegating-or-staking)

### How To use toolkit With This Branch

**Tool1. sendblocktomainchain**  which is to send child chain's block to mainchain(default url is http://localhost:6969/pchain) to refresh tx3/epoch, the command format is 
```
./pchain sendblocktomainchain --toolkitdir toolkit_dir
```

there should be one block.json and one priv_validator.json under toolkit_dir, and examples can be found under ./toolkitdir directory. block.json contains the block to send; and priv_validator.json could be any valid address, which is no need to be on-duty validator.

For example, to send block to the main chain 
```
./pchain sendblocktomainchain --toolkitdir /home/user/toolkit/.pchain
```

**Tool2. getblockwithtxs**  which is to get one chain's block to local, the command format is 
```
./pchain getblockwithtxs --chainid chain_id --blocknumber block_number --toolkitdir toolkit_dir 
```

if there is block with block_number on chain_id(the url will be http://localhost:6969/chain_id), one block_`block_number`.json file would be generated under toolkit_dir, which contains the json-format content of the block with its all transactions.

For example, to get block 13215 from child_0 
```
./pchain getblockwithtxs --chainid child_0 --blocknumber 13215 --toolkitdir /home/user/toolkit/.pchain
```

**Tool3. masswithdraw**  which is to send tx3(tx4 will be sent automatically for tx3) in batch to perform stress test, and this tool should run on local chain or test chain. the command format is 
```
./pchain masswithdraw --chainid chain_id --toolkitdir toolkit_dir --countpersec tx3number_per_second --totaltime total_seconds
```

with this command, the default mainchain url is http://localhost:6969/pchain, and the child chain is http://localhost:6969/chain_Id, there should be one UTC-xxx-address.json under toolkit_dir, and its example can be found under ./toolkitdir directory. this private key file contain one account who should have enough token(PI)  on chainurl for withdrawal.

For example, to send 10 tx3 to child_0 per-second, and last 5 seconds 
```
./pchain masswithdraw --chainid child_0 --toolkitdir /home/user/toolkit/.pchain --countpersec 10 --totaltime 5
```

**Tool4. crosschaintransfer**  which is to send cross chain transfer transaction. the command format is 
```
./pchain crosschaintransfer --fromchain fromchain_id --tochain tochain_id --amount amount --toolkitdir toolkit_dir
```

with this command, the fromchain's url is http://localhost:6969/fromchain_id, and the tochain's url is http://localhost:6969/tochain_Id, there should be one UTC-xxx-address.json under toolkit_dir, and its example can be found under 'toolkitdir' directory. this private key file contain one account who should have enough token(PI)  on fromchain_id for cross chain transfer.

For example, to send 1000 wei from pchain to child_0
```
./pchain crosschaintransfer --fromchain pchain --tochain child_0 --amount 1000 --toolkitdir /home/user/toolkit/.pchain
```

**Tool5. synchfromlocaldb**  which is to synchronize from local pchain's db.
```
./pchain synchfromlocaldb --chainId chain_id --roughCheckSync --gcmode=full --datadir data_dir --sourceDataDir source_data_dir
```

with this command, the chainId should be the chain's name which need to synchronize, roughCheckSync indicates if only verify roughly, sourceDataDir indicates the directory of local pchain's data.

For example, to synchronize data from local chain 'pchain'/'child_0' which data locates under /home/usr/code/master/.pchain
```
./pchain synchfromlocaldb --chainId pchain --roughCheckSync --gcmode=full --datadir /home/user/code/new_branch/.pchain --sourceDataDir /home/user/code/master/.pchain
```

Refer to [sync_from_local_db](https://github.com/pchain-org/pchain/blob/sync_from_local_db/README.md) to get more information

**Tool6. getEpochs**  which is to get all epochs' brief information. the command format is 
```
./pchain getEpochs --chainid chain_id --toolkitdir toolkit_dir
```

with this command, the chain's url is http://localhost:6969/chain_id, and the output file 'chain_id_epochs.txt' can be found under 'toolkit_dir' directory. 

For example, to get all epochs' brief information of child_0
```
./pchain getEpochs --chainid child_0 --toolkitdir /home/user/code/toolkit/.pchain
```

**Tool7. snapshot**  which is to snapshot one chain's data to save disk usage, and this chain of course can continue running on this snapshot. the command format is
```
./pchain snapshot --datadir data_dir --chainId chain_id
```

For example, to snapshot the main chain
```
./pchain snapshot --datadir /home/user/code/master/.pchain --chainId pchain
```

to snapshot the first official child chain
```
./pchain snapshot --chainId child_0 --datadir /home/user/code/master/.pchain
```

**Tool8. dump**  which is to dump accouts info to file
```
./pchain dump --datadir data_dir --chainId chain_id
```

For example, to dump the main chain
```
./pchain dump --datadir /home/user/code/master/.pchain --chainId pchain
```

to dump the first official child chain
```
./pchain dump --datadir /home/user/code/master/.pchain --chainId child_0
```