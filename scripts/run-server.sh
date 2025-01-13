#!/bin/bash

TAG=$1
if [ -z "$1" ]; then
  TAG="latest"
fi


# if we are in the scripts folder (likely mistake), move up one
if [ "${PWD##*/}" == "scripts" ]; then
  cd ..
fi

echo "Using image tag $TAG"

docker stop brotato-exporter-server | true
docker rm brotato-exporter-server | true

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
mkdir -p "$VOLUME_PATH/log"

# start the server and detach
docker run -d --name brotato-exporter-server \
  -v ${VOLUME_PATH}:/var/brotatoexporter -v ${VOLUME_PATH}/log:/var/log \
  -p 8081:8081 -p 8082:8082 \
  benwirth10/brotato-exporter:$TAG /exporter-server