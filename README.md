## Pchain

Official golang implementation of the Pchain protocol.

To get familiar with pchain, you could read about [Overview](https://github.com/pchain-org/pchain/wiki/Welcome-to-pchain)

## Installation Instructions for Ubuntu(Official recommendation 16.04)

Installing from PPA
```
sudo apt-get install software-properties-common
sudo add-apt-repository -y ppa:pchainorg/pchain
sudo apt-get update
sudo apt-get install pchain
```
You should now be able to check the different options and commands with 'pchain --help'.

Upgrade the latest version

```
sudo apt-get update
sudo apt-get dist-upgrade
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

And to build and run pchain under Windows, you could refer to [Build and start pchain](https://github.com/pchain-org/pchain/wiki/Build-and-start-pchain)

## Running pchain

please consult our
[CLI Wiki page](https://github.com/pchain-org/pchain/wiki/Command-Line-Options)


### Operating a private network with a single validator

Maintaining your own private network is more involved as a lot of configurations taken for granted in
the official networks need to be manually set up.

First, you'll need to create the eth_genesis state of your networks, which all nodes need to be aware of
and agree upon. This consists of a small JSON file (e.g. call it ` eth_genesis.json`):

```json
{
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
    "mixHash": "0x3030303030303030303030303030303030303030303030303030303030303030",
    "coinbase": "0x4cacbcbf218679dcc9574a90a2061bca4a8d8b6c",
    "alloc": {
            "4cacbcbf218679dcc9574a90a2061bca4a8d8b6c": {
                    "balance": "0x1ed09bead87c0378d8e6400000000",
                    "amount": "0x64"
            }
    },
    "number": "0x0",
    "gasUsed": "0x0",
    "parentHash": "0x3030303030303030303030303030303030303030303030303030303030303030"
}
```

The above fields should be fine for most purposes, although we'd recommend changing the `nonce` to
some random value so you prevent unknown remote nodes from being able to connect to you. If you'd
like to pre-fund some accounts for easier testing, you can populate the `alloc` field with account
configs:('amount' is the deposite stake for PDBFT)


```json
"alloc": {
            "4cacbcbf218679dcc9574a90a2061bca4a8d8b6c": {
                    "balance": "0x1ed09bead87c0378d8e6400000000",
                    "amount": "0x64"
            }
    }
```

Or you can create the 'eth_genesis.json' by the command below:(The system will create a new account with passphrase 'pchain',the balance is 10000000000000000000000000000000000 wei ,deposite amount is 100 wei)

```
 $ ./pchain --datadir .pchain init_eth_genesis "{10000000000000000000000000000000000, 100}"
```

Tips:The above command will also create a priv_validator.json with PDBFT information which is useful when you participate in PDBFT.
```json
{
        "address": "4CACBCBF218679DCC9574A90A2061BCA4A8D8B6C",  //Your Pchain address for PDBFT reward
        "consensus_priv_key": [
                4,         //PDBFT private key type
                "D8AF52E355CD070ED3401800CBC920B6E94F3C49B42808C3049BF7BDB1FA3B19"  //PDBFT private key
        ],
        "consensus_pub_key": [
                4,        //PDBFT private key type
                "085586D41F70435700850E19B7DE54B3E793C5EC4C6EC502D19030EF4F2122823E5A765E56CBA7B4C57E50561F77B022313C39895CA303F3C95D7B7282412F334778B95ACE046A79AEA4DB148334527250C8895AC5DB80459BF5D367236B59AF2DB5C0254E30A6D8CD1FA10AB8A5D872F5EBD312D3160D3E4DD496973BDC75E0"   //PDBFT public key
        ]
}
```

With the genesis state defined in the above JSON file, you'll need to initialize  pchain node
with it prior to starting it up to ensure all blockchain parameters are correctly set:

```
$ ./pchain init path/to/eth_genesis.json
```

#### Starting up your private node

```
$ ./pchain --datadir yourDataPath  --networkid yourNetworkId --port 30308 --mine --syncmode "full" 
```

### interfacing pchain nodes with RPC([CLI Wiki page](https://github.com/pchain-org/pchain/wiki/Command-Line-Options))

As a developer, sooner rather than later you'll want to start interacting with pchain 
network via your own programs and not manually through the console. To aid this, Pchain has built-in
support for a JSON-RPC based APIs ([standard APIs](https://github.com/pchain-org/pchain/wiki/JSON-RPC)

HTTP based JSON-RPC API options:

  * `--rpc` Enable the HTTP-RPC server
  * `--rpcaddr` HTTP-RPC server listening interface (default: "localhost",Do not set `0.0.0.0`,it is very dangerous.)
  * `--rpcport` HTTP-RPC server listening port (default: 6969)
  * `--rpcapi` API's offered over the HTTP-RPC interface (default: "eth,net,web3")
  * `--rpccorsdomain` Comma separated list of domains from which to accept cross origin requests (browser enforced)

You'll need to use your own programming environments' capabilities (libraries, tools, etc) to connect
via HTTP to a Pchain node configured with the above flags and you'll need to speak [JSON-RPC](https://www.jsonrpc.org/specification)
on all transports. You can reuse the same connection for multiple requests!

**Note: Please understand the security implications of opening up an HTTP/WS based transport before
doing so! Hackers on the internet are actively trying to subvert Pchain nodes with exposed APIs!
Further, all browser tabs can access locally running webservers, so malicious webpages could try to
subvert locally available APIs!** 


For more information, you could take a look at pchain's [wiki](https://github.com/pchain-org/pchain/wiki)
