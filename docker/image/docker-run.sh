SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"
source "$SCRIPT_DIR"/docker-env.sh

echo "Running $NODE_ID node group..."

# Remove this file while we're starting up.  Once it's written, it can be used as a flag
# to the outside world as to whether the container's processes are all fully running.
RUNNING_FILE="$SCRIPT_DIR/running"
rm -f "$RUNNING_FILE"

# If there's no data directory yet, it means we're starting from scratch.
if [ ! -d "$DATA_DIR" ]; then
    echo "Configuring node group..."
    /bin/bash "$SCRIPT_DIR"/docker-conf.sh
fi

# This is needed because in the long term, noms eats more than 256 file descriptors
ulimit -n 1024

# All commands are run out of the bin directory.
cd "$BIN_DIR" || exit 1

LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"

wait_port() {
    # Block until the given port becomes open.
    port="$1"
    echo "Waiting for port $port..."
    until nc -z localhost "$port" 2>/dev/null
    do
        :
    done
}

run_redis() {
    port="$1"
    data_dir="$2"
    echo "Running redis..."

    redis-server --dir "$data_dir" \
                 --port "$port" \
                 --save 60 1 \
                 >"$LOG_DIR/redis.log" 2>&1 &
    wait_port "$port"

    # Redis isn't really ready when it's port is open, wait for a ping to work.
    echo "Waiting for redis ping..."
    until [[ $(redis-cli -p "$port" ping) == "PONG" ]]
    do
        :
    done
}

run_noms() {
    port="$1"
    data_dir="$2"
    echo "Running noms..."

    ./noms serve --port="$port" "$data_dir" \
           >"$LOG_DIR/noms.log" 2>&1 &
    wait_port "$port"
}

run_node() {
    port="$1"
    redis_port="$2"
    noms_port="$3"
    echo "Running ndaunode..."

    ./ndaunode -spec http://localhost:"$noms_port" \
               -index localhost:"$redis_port" \
               -addr 0.0.0.0:"$port" \
               >"$LOG_DIR/ndaunode.log" 2>&1 &
    wait_port "$port"
}

run_tm() {
    p2p_port="$1"
    rpc_port="$2"
    node_port="$3"
    data_dir="$4"
    echo "Running tendermint..."

    CHAIN=ndau \
    ./tendermint node --home "$data_dir" \
                      --proxy_app tcp://localhost:"$node_port" \
                      --p2p.laddr tcp://0.0.0.0:"$p2p_port" \
                      --rpc.laddr tcp://0.0.0.0:"$rpc_port" \
                      --log_level="*:debug" \
                      >"$LOG_DIR/tendermint.log" 2>&1 &
    wait_port "$p2p_port"
    wait_port "$rpc_port"
}

run_ndauapi() {
    echo Running ndauapi...

    NDAUAPI_NDAU_RPC_URL=http://localhost:"$TM_RPC_PORT" \
    ./ndauapi >"$LOG_DIR/ndauapi.log" 2>&1 &
}

run_redis "$REDIS_PORT" "$REDIS_DATA_DIR"
run_noms "$NOMS_PORT" "$NOMS_DATA_DIR"
run_node "$NODE_PORT" "$REDIS_PORT" "$NOMS_PORT"
run_tm "$TM_P2P_PORT" "$TM_RPC_PORT" "$NODE_PORT" "$TM_DATA_DIR"
run_ndauapi

IDENTITY_FILE="$SCRIPT_DIR"/node-identity.tgz
if [ ! -f "$IDENTITY_FILE" ] && [ -z "$BASE64_NODE_IDENTITY" ]; then
    echo "Generating identity file..."

    cd "$DATA_DIR" || exit 1
    tar -czf "$IDENTITY_FILE" \
        tendermint/config/node_key.json \
        tendermint/config/priv_validator_key.json
fi

# Everything's up and running.  The outside world can poll for this file to know this.
touch "$RUNNING_FILE"

# If the INSANE_LOGGING environment varibale is set, tail will dump all output to stdout.
if [ !-z "$INSANE_LOGGING" ]; then
  tail "$LOG_DIR/*.log"-f &
fi

echo "Node group $NODE_ID is now running"

# Wait forever to keep the container alive.
while true; do sleep 86400; done
