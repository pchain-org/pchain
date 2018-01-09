#!/bin/sh

set -e

PWD=`pwd`
echo $PWD
workspace=$PWD
ethermintdir="$workspace/src/github.com/tendermint/ethermint"

GOPATH="$workspace"
export GOPATH
echo $GOPATH
cd "$ethermintdir"
echo `pwd`
make build
#go build -i -v -o $workspace/bin/geth ./cmd/geth
#go build -i -v  -o /home/ubuntu/csftan/hdchain/bin/geth ./cmd/geth
