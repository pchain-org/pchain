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

### Please Check Your Pchain Client Version (Latest:1.5.01)

```
$ pchain version
//1.5.01
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

### Synchronize chain from local db backup

This branch implements a command 'synchfromlocaldb' to make pchain can synchronize from local pchain's db.

When some new code are added to pchain and needed to process every block to verify, we need to synchronize from block zero.

But the network may be not accessible or its speed is very slow, then we can use this command to synchronize from local pchain's db if we have one.

For example, we have /home/user/code/master/.pchain as old pchain data, which contains blocks/txs of pchain/child_0, and our new code are added for chain 'child_0' in branch 'new_branch', then we can use this command to synchronize blocks and verify the code.

```
./pchain synchfromlocaldb --chainId child_0 --roughCheckSync --gcmode=full --datadir /home/user/code/new_branch/.pchain --sourceDataDir /home/user/code/master/.pchain
```

flags 'chainId' and 'roughCheckSync' are newly added for this command, the previous one indicates which chain to synchronize, the following one means it will avoid commit seals verification and only check state root every 5000 blocks when synchronizing.

The above command will synchronize chains both 'pchain' and 'child_0', after all the chain 'child_0' depends on the main chain 'pchain' to run.

If we just want to verify code for main chain, we only need to synchronize main chain's data, and we use the command like this:
```
./pchain synchfromlocaldb --chainId pchain --roughCheckSync --gcmode=full --datadir /home/user/code/new_branch/.pchain --sourceDataDir /home/user/code/master/.pchain
```
