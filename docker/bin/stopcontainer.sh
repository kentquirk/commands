#!/bin/bash

CONTAINER="$1"

if [ -z "$CONTAINER" ]; then
    CONTAINER=demonet-0
    echo "No container specified; using default: $CONTAINER"
fi

echo Stopping "$CONTAINER"...
docker container stop "$CONTAINER" 2>/dev/null
echo done
