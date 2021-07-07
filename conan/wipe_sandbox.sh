#!/bin/bash

ORIG="$(cd "$(dirname "$0")" || exit; pwd)"

# Stop after MAX_ATTEMPTS
MAX_ATTEMPTS=2
# retry after 48h
TTL_EVENTLOG=$((3600*24))

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
    local prevlogfile=~/pool_management/reset_${sandbox}.log.1
    local logfile=~/pool_management/reset_${sandbox}.log
    local eventlog=~/pool_management/reset_${sandbox}.events.log
    cd ~/pool_management/agnosticd/ansible

    # Keep previous log to help troubleshooting
    if [ -e "${logfile}" ]; then
        cp "${logfile}" "${prevlogfile}"
    fi

    if [ -e "${eventlog}" ]; then
        local age_eventlog=$(( $(date +%s) - $(date -r $eventlog +%s) ))
        # If last attempt was less than 24h (TTL_EVENTLOG) ago
        # and if it failed more than MAX_ATTEMPTS times, skip.
        if [ $age_eventlog -le $TTL_EVENTLOG ] && \
            [ $(wc -l $eventlog) -ge ${MAX_ATTEMPTS} ]; then
            echo "$(date) ${sandbox} Too many attemps, skipping"
            return
        fi
    fi


    echo "$(date) reset sandbox${s}" >> ~/pool_management/reset.log
    echo "$(date) reset sandbox${s}" >> $eventlog

    echo "$(date) ${sandbox} reset starting..."

    export ANSIBLE_COMMAND_WARNINGS=False
    export ANSIBLE_NO_TARGET_SYSLOG=True
    ansible-playbook -i localhost, \
                     -e _account_num=${s} \
                     ${ORIG}/reset_single.yml > ${logfile}

    if [ $? = 0 ]; then
        echo "$(date) ${sandbox} reset OK"
        rm $eventlog
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
