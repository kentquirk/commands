#!/bin/bash

CMDBIN_DIR="$(go env GOPATH)/src/github.com/ndau/commands/bin"
# shellcheck disable=SC1090
source "$CMDBIN_DIR"/env.sh

# This is needed because in the long term, noms eats more than 256 file descriptors
ulimit -n 1024

for node_num in $(seq 0 "$HIGH_NODE_NUM"); do
    output_name="$CMDBIN_DIR/procmon-$node_num"
    export NOMS_PORT_NUM=$((NOMS_PORT + node_num))
    export NODE_PORT_NUM=$((NODE_PORT + node_num))
    export REDIS_PORT_NUM=$((REDIS_PORT + node_num))
    export TM_P2P_PORT_NUM=$((TM_P2P_PORT + node_num))
    export TM_RPC_PORT_NUM=$((TM_RPC_PORT + node_num))
    export TM_RPC_PORT_NUM=$((TM_RPC_PORT + node_num))
    export NDAUAPI_PORT_NUM=$((NDAUAPI_PORT + node_num))
    NODE_NUM=$node_num ./procmon --configfile procmon.toml >"$output_name.log" 2>&1 &
    echo starting procmon $node_num as PID $!
done

echo "done."
