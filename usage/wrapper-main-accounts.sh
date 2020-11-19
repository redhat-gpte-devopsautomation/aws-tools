#!/usr/bin/env bash
set -euo pipefail

mkdir -p ~/aws-usage-data
cd ~/aws-usage-data
export PROMETHEUS_GATEWAY=http://localhost:9091
TIMEOUT=300
threads=${1:-8}
rush \
    --propagate-exit-status=false \
    --immediate-output \
    -j ${threads} \
    --timeout ${TIMEOUT} \
    "bash -c 'export AWS_PROFILE={1}; aws-usage > /dev/null'" <<EOF
gpte
dev
infra
opentlc
events
EOF
