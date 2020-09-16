welcome to our lastest [instruction](https://pchaindoc.readthedocs.io/en/latest/index.html).
## Pchain

[中文README](https://github.com/pchain-org/pchain/wiki/README_CN)

Official golang implementation of the Pchain protocol.

To get familiar with pchain, you could read about [Overview](https://github.com/pchain-org/pchain/wiki/Welcome-to-pchain)

## Installation Instructions for Ubuntu(Official recommendation 16.04)

Installing from PPA, or [install from release](https://github.com/pchain-org/pchain/wiki/Install-pchain-from-release)(If you wanna become PCHAIN validator, we recommend you to install from release)
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

you can use our Homebrew tap to install pchain. If you don't have Homebrew,[install it first](https://brew.sh/)

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
just for developer,if you have installed pchain client,just skip this.

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

If you want to build and run pchain under Windows(Not recommended), you could refer to [Build and start pchain](https://github.com/pchain-org/pchain/wiki/Build-and-start-pchain)

If you want to know more about our Command Line Options,please consult our 
[CLI Wiki page](https://github.com/pchain-org/pchain/wiki/Command-Line-Options)

### Please Check Your Pchain Client Version (Latest:1.3.3)

```
$ pchain version
//1.3.4
```

### Sync and run testnet
If you wanna testing your smart contract, it's better to deploy it on testnet first. 
You can follow [this guide](https://github.com/pchain-org/pchain/wiki/How-to-sync-and-run-pchain's-testnet) to run pchain's testnet.

You can [Get free tPI](https://testnet.pchain.org/vfaucet.html) from our testnet.

### Full Node On The Pchain Main Network

```
$ pchain
```
You can set your own data directory by '--datadir yourOwnDirectory'
If you want to open RPC,just add the options '--rpc' and '--rpcapi "db,eth,net,web3,admin,tdm,miner,personal,chain,txpool" '. More details,please read [How To Interact With Pchain](https://github.com/pchain-org/pchain#how-to-interact-with-pchainrpc-or-ipc-console)

you can use 'nohup' and add '&' at the end of command above,Pchain client will run in the background

```
$ nohup pchain &
```

| Platform | Default Datadir Directory |
|:-------|:--------|
| Linux | ~/.pchain |
| Mac | ~/Library/Pchain|
| Windows | %APPDATA%\Pchain|

### Docker quick start

One of the quickest ways to get Pchain up and running on your machine is by using Docker:
```
docker run -d --name pchain-node -v ~/pchain/.pchain:/.pchain \
           -p 6969:6969 -p 30308:30308 \
           pchain/client-go --datadir=/.pchain
```
This will start pchain just as the above command does. It will also create a persistent volume in your home directory for saving your blockchain as well as map the default ports.

Do not forget `--rpcaddr=0.0.0.0 --rpc --rpcapi=eth,web3,admin,tdm,miner,personal,chain,txpool,del`, if you want to access RPC from other containers and/or hosts. By default, pchain binds to the local interface and RPC endpoints is not accessible from the outside.

### How To Interact With Pchain(RPC OR IPC CONSOLE)

Please Read The Wiki [How To Interact With Pchain](https://github.com/pchain-org/pchain/wiki/How-To-Interact-With-Pchain)

### How To Become A Validator

Validator is a node participating in PCHAIN for the verification and consensus process of generating the block. 

Please Read The Wiki [How To Become A Validator](https://github.com/pchain-org/pchain/wiki/How-To-Become-A-Validator)

### How to Become A Candidate

Candidate is a node competing Validator with the support of delegators and the PCHAIN delegation mechanism.

Please Read The Wiki [How to Become a Candidate](https://github.com/pchain-org/pchain/wiki/How-to-Become-a-Candidate)

### How To Delegate

It is the process that a Delegator stake its token to a/several specific Validator(s).

Please Read The Wiki [How To Delegate](https://github.com/pchain-org/pchain/wiki/How-To-Delegate)



### Operating a private network with a single validator

Please Read The Wiki [Operating A Private Network With A Single Validator](https://github.com/pchain-org/pchain/wiki/Operating-A-Private-Network-With-A-Single-Validator)

### Run pchain testnet
Please read this [guide](https://github.com/pchain-org/pchain/wiki/How-to-sync-and-run-pchain's-testnet)
