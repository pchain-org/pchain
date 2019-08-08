#!/bin/sh

set -e

PWD=`pwd`
echo $PWD
workspace=$PWD
pchaindir="$workspace/src/github.com/pchain"

GOPATH="$workspace"
export GOPATH
echo $GOPATH
cd "$pchaindir"
echo `pwd`
#make build
go build -i -v -o $workspace/bin/pchain ./cmd
