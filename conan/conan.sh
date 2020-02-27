#!/bin/bash

set -u -o pipefail

##############
# conf
##############

# Number of aws-nuke to run in parallel
threads=10

# Pause between each iteration that gets the list of sandboxes to delete
poll_interval=60

##############

ORIG="$(cd "$(dirname "$0")" || exit; pwd)"

pre_checks() {
    for c in sandbox-list \
             rush \
             sandbox_reset.sh; do
        if ! command -v $c; then
            echo "'${c}' command not found"
            exit 2
        fi
    done
}

pre_checks

cd ${ORIG}

while true; do

    sandbox-list --to-delete --no-headers \
        | rush -j ${threads} './wipe_sandbox.sh {1}'

    sleep ${poll_interval}
done
