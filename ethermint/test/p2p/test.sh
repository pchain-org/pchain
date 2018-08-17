#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=local_testnet

cd $GOPATH/src/github.com/pchain/ethermint

# start the testnet on a local network
# test p2p
bash test/p2p/local_testnet.sh $DOCKER_IMAGE $NETWORK_NAME
