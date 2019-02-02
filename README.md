## Pchain

Official golang implementation of the Pchain protocol.

To get familiar with pchain, you could read about [Overview](https://github.com/pchain-org/pchain/wiki/Welcome-to-pchain)

## Installation Instructions for Ubuntu(Official recommendation 16.04)

Installing from PPA
```
sudo apt update
sudo apt install software-properties-common
sudo add-apt-repository -y ppa:pchainorg/pchain
sudo apt update
sudo apt install pchain
```
You should now be able to check the different options and commands with 'pchain --help'.

Upgrade the latest version

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

Upgrade the latest version

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
git clone https://github.com/pchain-org/pchain.git
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

### Full Node On The Pchain Main Network

Pchain Main network will come soon.


### Full Node On the Pchain Test Network 

```
$ pchain --testnet --networkid=10000 --port=30201 
```

You can set your own data directory by '--datadir yourOwnDirectory'

If you want to open RPC,just add the options '--rpc' and '--rpcapi "db,eth,net,web3,admin,tdm,miner,personal,chain,txpool" '. More details,please read [How To Interact With Pchain](https://github.com/pchain-org/pchain#how-to-interact-with-pchainrpc-or-ipc-console)

you can add '&' at the end of command above,Pchain client will run in the background

```
$ pchain --testnet --networkid=10000 --port=30201  &
```

| Platform | Default Datadir Directory |
|:-------|:--------|
| Linux | ~/.pchain |
| Mac | ~/Library/Pchain|
| Windows | %APPDATA%\Pchain|

Specifying the `--testnet` flag however will reconfigure your pchain instance a bit:

 * Instead of using the default data directory , pchain will nest
   itself one level deeper into a `testnet` subfolder (`<Default_Datadir>/testnet`). 

 * Instead of connecting the main pchain network, the client will connect to the test network,
   which uses different P2P bootnodes, different network IDs and genesis states.

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

