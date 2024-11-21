#!/bin/bash

TAG=${1:-latest}

echo "Using image tag $TAG"

# if we are in the scripts folder (likely mistake), move up one
if [ "${PWD##*/}" == "scripts" ]; then
  cd ..
fi

docker stop mod-user-create | true
docker rm mod-user-create | true

# check if server is running and exit if so
if [ "$(docker ps -q -f name=brotato-exporter-server)" ]; then
  echo "Stop server before running"
  exit 1
fi

set -e

VOLUME_PATH=`pwd`/var-brotatoexporter

# Check if 'cygpath' is available
if command -v cygpath >/dev/null 2>&1; then
  # On Windows using Git Bash, Cygwin, or MSYS
  VOLUME_PATH="$(cygpath -w "$VOLUME_PATH")"
  VOLUME_PATH="${VOLUME_PATH//\\//}"
fi

echo "Using volume path $VOLUME_PATH"
mkdir -p "$VOLUME_PATH"

# create a user interactively
docker run -it --name mod-user-create \
  --entrypoint ./mod-user-create \
  -v ${VOLUME_PATH}:/var/brotatoexporter \
  benwirth10/brotato-exporter:$TAG