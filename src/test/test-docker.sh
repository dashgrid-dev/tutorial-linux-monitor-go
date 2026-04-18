#!/bin/bash
# Local test: cross-compile and run the monitor in a Docker linux container
# config.test.yaml. Requires: docker, a reachable Dashgrid server at the
# api_host set in config.test.yaml (localhost:* is remapped to the Mac host).
#
# Usage: bash src/test/test-docker.sh     (Ctrl-C to stop)

set -euo pipefail
cd "$(dirname "$0")"

if [ ! -f config.test.yaml ]; then
  echo "Error: config.test.yaml not found in $(pwd)"
  echo "Copy the example and fill in your API key + bucket IDs:"
  echo "  cp config.test.yaml.example config.test.yaml"
  exit 1
fi

echo ">>> Building linux/amd64 binary..."
(cd ../monitor && GOOS=linux GOARCH=amd64 go build -o ../test/dashgrid-monitor-linux-amd64 .)

echo ">>> Running in docker (localhost -> host.docker.internal)..."
docker run --rm \
  -e DASHGRID_CONFIG=/monitor/config.test.yaml \
  --add-host=localhost:host-gateway \
  -v "$PWD:/monitor" -w /monitor \
  debian:stable-slim sh -c 'apt-get update -qq && apt-get install -y -qq --no-install-recommends ca-certificates >/dev/null && ./dashgrid-monitor-linux-amd64'
