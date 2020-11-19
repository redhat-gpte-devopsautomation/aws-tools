#!/usr/bin/env bash
set -euo pipefail
##############

ORIG="$(cd "$(dirname "$0")" || exit; pwd)"
export PATH=${HOME}/bin:${PATH}

# Max time for running aws-usage per account
TIMEOUT=300

# Sandboxes
# Number of processes to run in parallel
threads=${1:-8}

pre_checks() {
    for c in sandbox-list \
             aws-usage \
             rush ; do
        if ! command -v $c &>/dev/null; then
            echo "'${c}' command not found"
            exit 2
        fi
    done
}

pre_checks

mkdir -p ~/aws-usage-data
cd ~/aws-usage-data
export PROMETHEUS_GATEWAY=http://localhost:9091

# Load ~/.aws/config
export AWS_SDK_LOAD_CONFIG=true

# Delete unused sandboxes from the push gateway

(AWS_PROFILE=pool-manager sandbox-list --csv --all) \
    | awk -F, '$2 == "true" {print $1}' \
    | rush \
    --propagate-exit-status=false \
    --timeout ${TIMEOUT} \
    --immediate-output \
    -j ${threads} \
    "bash -c 'export AWS_PROFILE={1}; aws-usage --reset > /dev/null'"

(AWS_PROFILE=pool-manager sandbox-list --csv --all) \
    | awk -F, '$2 == "false" {print $1}' \
    | rush \
    --propagate-exit-status=false \
    --timeout ${TIMEOUT} \
    --immediate-output \
    -j ${threads} \
    "bash -c 'export AWS_PROFILE={1}; aws-usage --profile --s3=false --addresses=false > /dev/null'"
