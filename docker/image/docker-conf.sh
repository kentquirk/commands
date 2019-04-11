#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"
source "$SCRIPT_DIR"/docker-env.sh

SNAPSHOT_DIR="$SCRIPT_DIR/snapshot"
mkdir -p "$SNAPSHOT_DIR"
SNAPSHOT_FILE="${SNAPSHOT_URL##*/}"

echo "Fetching $SNAPSHOT_FILE..."
wget -O "$SNAPSHOT_DIR/$SNAPSHOT_FILE" "$SNAPSHOT_URL"

echo "Extracting $SNAPSHOT_FILE..."
cd "$SNAPSHOT_DIR" || exit 1
tar -xf "$SNAPSHOT_FILE"

echo "Validating $SNAPSHOT_DIR..."
if [ ! -d "$SNAPSHOT_DIR" ]; then
    echo "Could not find snapshot directory: $SNAPSHOT_DIR"
    exit 1
fi
SNAPSHOT_DATA_DIR="$SNAPSHOT_DIR/data"
if [ ! -d "$SNAPSHOT_DATA_DIR" ]; then
    echo "Could not find data directory: $SNAPSHOT_DATA_DIR"
    exit 1
fi
SNAPSHOT_NOMS_DATA_DIR="$SNAPSHOT_DATA_DIR/noms"
if [ ! -d "$SNAPSHOT_NOMS_DATA_DIR" ]; then
    echo "Could not find noms data directory: $SNAPSHOT_NOMS_DATA_DIR"
    exit 1
fi
SNAPSHOT_REDIS_DATA_DIR="$SNAPSHOT_DATA_DIR/redis"
if [ ! -d "$SNAPSHOT_REDIS_DATA_DIR" ]; then
    echo "Could not find redis data directory: $SNAPSHOT_REDIS_DATA_DIR"
    exit 1
fi
SNAPSHOT_TENDERMINT_HOME_DIR="$SNAPSHOT_DATA_DIR/tendermint"
if [ ! -d "$SNAPSHOT_TENDERMINT_HOME_DIR" ]; then
    echo "Could not find tendermint home directory: $SNAPSHOT_TENDERMINT_HOME_DIR"
    exit 1
fi
SNAPSHOT_TENDERMINT_CONFIG_DIR="$SNAPSHOT_TENDERMINT_HOME_DIR/config"
if [ ! -d "$SNAPSHOT_TENDERMINT_CONFIG_DIR" ]; then
    echo "Could not find tendermint config directory: $SNAPSHOT_TENDERMINT_CONFIG_DIR"
    exit 1
fi
SNAPSHOT_TENDERMINT_GENESIS_FILE="$SNAPSHOT_TENDERMINT_CONFIG_DIR/genesis.json"
if [ ! -f "$SNAPSHOT_TENDERMINT_GENESIS_FILE" ]; then
    echo "Could not find tendermint genesis file: $SNAPSHOT_TENDERMINT_GENESIS_FILE"
    exit 1
fi

# Move the snapshot data dir where the applications expect it, then remove the temp snapshot dir.
mv "$SNAPSHOT_DATA_DIR" "$DATA_DIR"
rm -rf $SNAPSHOT_DIR

check_identity_files() {
  # check to make sure the identity files are where they shold be.
  if [ ! -f "$DATA_DIR/tendermint/config/priv_validator_key.json" ] || \
     [ ! -f "$DATA_DIR/tendermint/config/node_key.json" ]; then
     echo "Identity files were not found in the expected location"
     find "$DATA_DIR/tendermint/config"
     exit 1
  fi
}

# If we have an environment variable that defines identities, do not use an identity file.
if [ ! -z "$BASE64_NODE_IDENTITY" ]; then
  # echo the environment variable, decode the base64, and unzip into the files.
  echo "Using node identity environment variables"
  cd "$DATA_DIR" || exit 1
  echo -n "$BASE64_NODE_IDENTITY" | base64 -d | tar xfvz -
  check_identity_files
else
  # If we have a node identity file, extract its contents to the data dir.
  # It'll blend with other files already there from the snapshot.
  IDENTITY_FILE=node-identity.tgz
  if [ -f "$SCRIPT_DIR/$IDENTITY_FILE" ]; then
      echo "Using existing node identity..."
      # Copy, don't move, in case the node operator wants to copy it out again later.
      # Its presence also prevents us from generating it later.
      cp "$SCRIPT_DIR/$IDENTITY_FILE" "$DATA_DIR"
      cd "$DATA_DIR" || exit 1
      tar -xf "$IDENTITY_FILE"
      check_identity_files
  else
      # When we start without a node identity, we generate one so the node operator can restart
      # this node later, having the same identity every time.
      echo "No node identity found; a new node identity will be generated"
  fi
fi

# Tendermint complains if this file isn't here, but it can be empty json.
pvs_dir="$DATA_DIR/tendermint/data"
pvs_file="$pvs_dir/priv_validator_state.json"
if [ ! -f "$pvs_file" ]; then
  mkdir -p "$pvs_dir"
  echo "{}" > "$pvs_file"
fi

# Make directories that don't get created elsewhere.
mkdir -p "$NODE_DATA_DIR"
mkdir -p "$LOG_DIR"

cd "$BIN_DIR" || exit 1

echo Configuring tendermint...
# This will init all the config for the current container, leaving genesis.json alone.
./tendermint init --home "$TM_DATA_DIR"
sed -i -E \
    -e 's/^(create_empty_blocks = .*)/# \1/' \
    -e 's/^(create_empty_blocks_interval =) (.*)/\1 "300s"/' \
    -e 's/^(addr_book_strict =) (.*)/\1 false/' \
    -e 's/^(allow_duplicate_ip =) (.*)/\1 true/' \
    -e 's/^(moniker =) (.*)/\1 "'"$NODE_ID"'"/' \
    "$TM_DATA_DIR/config/config.toml"
sed -i -E \
    -e 's|^(persistent_peers =) (.*)|\1 "'"$PERSISTENT_PEERS"'"|' \
    "$TM_DATA_DIR/config/config.toml"

echo Configuration complete
