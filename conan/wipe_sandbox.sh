#!/bin/bash

ORIG="$(cd "$(dirname "$0")" || exit; pwd)"

prepare_workdir() {
    mkdir -p ~/pool_management

    if [ ! -d ~/pool_management/agnosticd ]; then
        git clone https://github.com/redhat-cop/agnosticd.git \
            ~/pool_management/agnosticd
    fi
}

sandbox_disable() {
    local sandbox=$1
    read -r -d '' data << EOM
  {
        ":av":      {"BOOL": false}
  }
EOM

    aws --profile pool-manager \
        --region us-east-1 \
        dynamodb update-item \
        --table-name accounts \
        --key "{\"name\": {\"S\": \"${sandbox}\"}}" \
        --update-expression "SET available = :av" \
        --expression-attribute-values "${data}"
}

sandbox_reset() {
    local s=${1##sandbox}
    local logfile=~/pool_management/reset_${sandbox}.log
    cd ~/pool_management/agnosticd/ansible

    echo "$(date) reset sandbox${s}" >> ~/pool_management/reset.log

    echo "$(date) ${sandbox} reset starting..."
    ansible-playbook -i localhost, \
                     -e _account_num=${s} \
                     ${ORIG}/reset_single.yml > ${logfile}

    if [ $? = 0 ]; then
        echo "$(date) ${sandbox} reset OK"
    else
        echo "$(date) ${sandbox} reset FAILED. See ${logfile}" >&2
        exit 3
    fi
}

sandbox=$1
if [ -z "${sandbox}" ]; then
    echo "sandbox not provided"
    exit 2
fi

prepare_workdir

sandbox_disable "${sandbox}"

sandbox_reset "${sandbox}"
