#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"

SNAPSHOT_BASE_URL="https://s3.amazonaws.com/ndau-snapshots"
INTERNAL_P2P_PORT=26660
INTERNAL_RPC_PORT=26670
INTERNAL_NDAUAPI_PORT=3030
LOG_FORMAT=json
LOG_LEVEL=info

if [ -z "$1" ] || \
   [ -z "$2" ] || \
   [ -z "$3" ] || \
   [ -z "$4" ] || \
   # $5 can be empty
   # $6 can be empty
   [ -z "$7" ]
   # $8 is optional
then
    echo "Usage:"
    echo "  ./runcontainer.sh" \
         "CONTAINER P2P_PORT RPC_PORT NDAUAPI_PORT PEERS SNAPSHOT IDENTITY"
    echo
    echo "Arguments:"
    echo "  CONTAINER     Name to give to the container to run"
    echo "  P2P_PORT      External port to map to the internal P2P port for the blockchain"
    echo "  RPC_PORT      External port to map to the internal RPC port for the blockchain"
    echo "  NDAUAPI_PORT  External port to map to the internal ndauapi port"
    echo "  PEERS_P2P     Comma-separated list of persistent peers on the network to join"
    echo "                  Each peer should be of the form IP_OR_DOMAIN_NAME:PORT"
    echo "  PEERS_RPC     Comma-separated list of the same peers for RPC connections"
    echo "                  Each peer should be of the form PROTOCOL://IP_OR_DOMAIN_NAME:PORT"
    echo "  SNAPSHOT      Name of the snapshot to use as a starting point for the node group"
    echo
    echo "Optionsl:"
    echo "  IDENTITY      node-identity.tgz file from a previous snaphot or initial container run"
    echo "                If present, the node will use it to configure itself when [re]starting"
    echo "                If missing, the node will generate a new identity for itself"
    exit 1
fi
CONTAINER="$1"
P2P_PORT="$2"
RPC_PORT="$3"
NDAUAPI_PORT="$4"
PEERS_P2P="$5"
PEERS_RPC="$6"
SNAPSHOT="$7"
IDENTITY="$8"

if [[ "$CONTAINER" == *"/"* ]]; then
    # This is because we use a sed command inside the container and slashes confuse it.
    echo "Container name $CONTAINER cannot contain slashes"
    exit 1
fi

echo "Container: $CONTAINER"

if [ ! -z "$(docker container ls -a -q -f name=$CONTAINER)" ]; then
    echo "Container already exists: $CONTAINER"
    echo "Use restartcontainer.sh to restart it, or use removecontainer.sh to remove it first"
    exit 1
fi

if [ ! -z "$IDENTITY" ] && [ ! -f "$IDENTITY" ] ; then
    echo "Cannot find node identity file: $IDENTITY"
    exit 1
fi

echo "P2P port: $P2P_PORT"
echo "RPC port: $RPC_PORT"
echo "ndauapi port: $NDAUAPI_PORT"

test_local_port() {
    port="$1"

    $(nc -G 1 -z localhost "$port" 2>/dev/null)
    if [ "$?" = 0 ]; then
        echo "Port at $ip:$port is already in use"
        exit 1
    fi
}

test_local_port "$P2P_PORT"
test_local_port "$RPC_PORT"
test_local_port "$NDAUAPI_PORT"

test_peer() {
    ip="$1"
    port="$2"

    if [ -z "$ip" ] || [ -z "$port" ]; then
        echo "Missing p2p ip or port: ip=($ip) port=($port)"
        exit 1
    fi

    echo "Testing connection to peer $ip:$port..."
    $(nc -G 5 -z "$ip" "$port")
    if [ "$?" != 0 ]; then
        echo "Could not reach peer"
        exit 1
    fi
}

get_peer_id() {
    protocol="$1"
    ip="$2"
    port="$3"

    if [ -z "$protocol" ] || [ -z "$ip" ] || [ -z "$port" ]; then
        echo "Missing rpc protocol, ip or port: protocol=($protocol) ip=($ip) port=($port)"
        exit 1
    fi

    echo "Getting peer info for $ip:$port..."
    PEER_ID=$(curl -s --connect-timeout 5 "$protocol://$ip:$port/status" | \
                  jq -r .result.node_info.id)
    if [ -z "$PEER_ID" ]; then
        echo "Could not get peer id"
        exit 1
    fi
    echo "Peer id: $PEER_ID"
}

# Split the peers list by comma, then by colon.  Build up the "id@ip:port" persistent peer list.
persistent_peers=()
peers_p2p=()
peers_rpc=()
IFS=',' read -ra PEER <<< "$PEERS_P2P"
for i in "${PEER[@]}"; do
    peers_p2p+=("$i")
done
IFS=',' read -ra PEER <<< "$PEERS_RPC"
for i in "${PEER[@]}"; do
    peers_rpc+=("$i")
done
len="${#peers_p2p[@]}"
if [ "$len" != "${#peers_rpc[@]}" ]; then
    echo "The length of P2P and RPC peers must match"
    exit 1
fi
for peer in $(seq 0 $((len - 1))); do
    pieces=()
    IFS=':' read -ra pair <<< "${peers_p2p[$peer]}"
    for i in "${pair[@]}"; do
        pieces+=("$i")
    done
    p2p_ip="${pieces[0]}"
    p2p_port="${pieces[1]}"

    test_peer "$p2p_ip" "$p2p_port"

    pieces=()
    IFS=':' read -ra pair <<< "${peers_rpc[$peer]}"
    for i in "${pair[@]}"; do
        pieces+=("$i")
    done
    rpc_protocol="${pieces[0]}"
    rpc_ip="${pieces[1]}"
    rpc_port="${pieces[2]}"

    # Since we split on colon, the double-slash is stuck to the ip.  Remove it.
    rpc_ip="${rpc_ip:2}"

    PEER_ID=""
    get_peer_id "$rpc_protocol" "$rpc_ip" "$rpc_port"
    persistent_peers+=("tcp://$PEER_ID@$p2p_ip:$p2p_port")
done

# Join array elements together by a delimiter.  e.g. `join_by , (a b c)` returns "a,b,c".
join_by() { local IFS="$1"; shift; echo "$*"; }

PERSISTENT_PEERS=$(join_by , "${persistent_peers[@]}")
echo "Persistent peers: '$PERSISTENT_PEERS'"

echo "Snapshot: $SNAPSHOT"

# Stop the container if it's running.  We can't run or restart it otherwise.
"$SCRIPT_DIR"/stopcontainer.sh "$CONTAINER"

echo Silencing warning about Transparent Huge Pages when redis-server runs...
docker run --rm -it --privileged --pid=host debian nsenter -t 1 -m -u -n -i \
       sh -c 'echo never > /sys/kernel/mm/transparent_hugepage/enabled'
docker run --rm -it --privileged --pid=host debian nsenter -t 1 -m -u -n -i \
       sh -c 'echo never > /sys/kernel/mm/transparent_hugepage/defrag'

echo "Creating container..."
# Some notes about the params to the run command:
# - Using --sysctl silences a warning about TCP backlog when redis runs.
# - Set your own HONEYCOMB_* env vars ahead of time to enable honeycomb logging.
docker create \
       -p "$P2P_PORT":"$INTERNAL_P2P_PORT" \
       -p "$RPC_PORT":"$INTERNAL_RPC_PORT" \
       -p "$NDAUAPI_PORT":"$INTERNAL_NDAUAPI_PORT" \
       --name "$CONTAINER" \
       -e "HONEYCOMB_DATASET=$HONEYCOMB_DATASET" \
       -e "HONEYCOMB_KEY=$HONEYCOMB_KEY" \
       -e "LOG_FORMAT=$LOG_FORMAT" \
       -e "LOG_LEVEL=$LOG_LEVEL" \
       -e "NODE_ID=$CONTAINER" \
       -e "PERSISTENT_PEERS=$PERSISTENT_PEERS" \
       -e "SNAPSHOT_URL=$SNAPSHOT_BASE_URL/$SNAPSHOT.tgz" \
       --sysctl net.core.somaxconn=511 \
       ndauimage 

IDENTITY_FILE=node-identity.tgz
if [ ! -z "$IDENTITY" ]; then
    echo "Copying node identity file to container..."
    docker cp "$IDENTITY" "$CONTAINER:/image/$IDENTITY_FILE"
fi

echo "Starting container..."
docker start "$CONTAINER"

echo "Waiting for the node to fully spin up..."
until docker exec "$CONTAINER" test -f /image/running 2>/dev/null
do
    :
done

# In the case no node identity was passed in, wait for it to generate one then copy it out.
# It's important that node operators keep the node-identity.tgz file secure.
if [ -z "$IDENTITY" ]; then
    # We can copy the file out now since we waited for the node to full spin up above.
    OUT_FILE="$SCRIPT_DIR/node-identity-$CONTAINER.tgz"
    docker cp "$CONTAINER:/image/$IDENTITY_FILE" "$OUT_FILE"

    echo
    echo "The node identity has been generated and copied out of the container here:"
    echo "  $OUT_FILE"
    echo
    echo "You can always get it at a later time by running the following:"
    echo "  docker cp $CONTAINER:/image/$IDENTITY_FILE $IDENTITY_FILE"
    echo "It can be used to restart this container with the same identity it has now"
    echo "Keep it secret; keep it safe"
    echo
fi

echo done
