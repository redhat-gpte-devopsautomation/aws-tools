#!/bin/bash

disable_sandbox() {
    local sandbox=$1
    read -r -d '' data << EOM
  {
        ":av":      {"BOOL": false}
  }
EOM

    aws --profile pool-manager-admin \
        --region us-east-1 \
        dynamodb update-item \
        --table-name accounts \
        --key "{\"name\": {\"S\": \"${sandbox}\"}}" \
        --update-expression "SET available = :av" \
        --expression-attribute-values "${data}"
}

sandbox=$1

[ -z "${sandbox}" ] && return
mkdir -p ~/pool_management
logfile=~/pool_management/reset_${sandbox}.log

disable_sandbox "${sandbox}"

sandbox_reset.sh "${sandbox}" > $logfile


if [ $? = 0 ]; then
    echo "$(date) ${sandbox} reset OK"
else
    echo "$(date) ${sandbox} reset FAILED. See ${logfile}" >&2
fi
