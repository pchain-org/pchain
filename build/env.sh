#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
pchaindir="$workspace/src/github.com"
if [ ! -L "$pchaindir/pchain" ]; then
    mkdir -p "$pchaindir"
    cd "$pchaindir"
    ln -s ../../../../. pchain
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"

export GOPATH

# Run the command inside the workspace.
cd "$pchaindir/pchain"
PWD="$pchaindir/pchain"

# build pchain client
go build -o $root/bin/pchain ./cmd/
