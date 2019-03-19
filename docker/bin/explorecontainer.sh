#!/bin/bash

CONTAINER="$1"
if [ -z "$CONTAINER" ]; then
    echo "Usage:"
    echo "  ./explorecontainer.sh CONTAINER"
    exit 1
fi

# This starts a shell inside the ndau image.
docker exec -it "$CONTAINER" /bin/sh
