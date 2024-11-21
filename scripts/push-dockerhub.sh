#!/bin/bash

if [ -z "$1" ]; then
  echo "Usage: $0 <tag>"
  exit 1
fi

# if we are in the scripts folder (likely mistake), move up one
if [ "${PWD##*/}" == "scripts" ]; then
  cd ..
fi

# assumes login has already been done
docker build -t benwirth10/brotato-exporter:latest .
docker tag benwirth10/brotato-exporter:latest benwirth10/brotato-exporter:$1

docker push benwirth10/brotato-exporter:latest
docker push benwirth10/brotato-exporter:$1