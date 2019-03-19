#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"

NODE1_CONTAINER=demonet-1
NODE1_CHAOS_P2P=26662
NODE1_CHAOS_RPC=26672
NODE1_NDAU_P2P=26663
NODE1_NDAU_RPC=26673

NODE1_CONTAINER=demonet-2
NODE2_CHAOS_P2P=26664
NODE2_CHAOS_RPC=26674
NODE2_NDAU_P2P=26665
NODE2_NDAU_RPC=26675

cd "$SCRIPT_DIR"/../bin || exit 1

./runcontainer.sh \
    "$NODE1_CONTAINER" \
    "$NODE1_CHAOS_P2P" \
    "$NODE1_CHAOS_RPC" \
    "$NODE1_NDAU_P2P" \
    "$NODE1_NDAU_RPC"

./runcontainer.sh \
    "$NODE2_CONTAINER" \
    "$NODE2_CHAOS_P2P" \
    "$NODE2_CHAOS_RPC" \
    "$NODE2_NDAU_P2P" \
    "$NODE2_NDAU_RPC"
