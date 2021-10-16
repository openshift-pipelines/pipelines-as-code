#!/usr/bin/env bash
# Replay a pac pipelinerun grabbing its payload and env from the output
# need tkn, fzf
set -euf

TMPD=$(mktemp -d /tmp/.run-server-replay-XXXX)
clean() { rm -rf ${TMPD} ;}
trap clean EXIT

if [[ ${1:-""} == -l ]];then
    arg="-L"
elif [[ -n ${1:-""} ]];then
    arg="${1}"
else
   pr=$(tkn pr ls --no-headers|fzf  -1)
   [[ -z ${pr} ]] && exit 0
   arg=$(echo ${pr}|awk '{print $1}')
   echo "Selected ${arg}"
fi
tkn pr logs ${arg} --prefix=false 2>/dev/null > ${TMPD}/last
[[ -s ${TMPD}/last ]] || { echo "payload could not be found"; exit 1 ;}

sed '/PAC/,$ { d;}' ${TMPD}/last > ${TMPD}/payload.json
[[ -s ${TMPD}/payload.json ]] || { echo "payload json could not be found"; exit 1 ;}


grep -e "PAC_.[a-zA-Z0-9_-]*=" ${TMPD}/last |sed -e 's/=\(.*\)/="\1"/' -e 's/^/export /' > ${TMPD}/env 
[[ -s ${TMPD}/env ]] || { echo "payload env could not be found"; exit 1 ;}

source $TMPD/env

PAC_PAYLOAD_FILE="/tmp/pac-payload-${PAC_WEBVCS_TYPE}-${PAC_WEBHOOK_TYPE}-${PAC_TRIGGER_TARGET}.json"
cat ${TMPD}/payload.json |tee ${PAC_PAYLOAD_FILE}
echo PAC_PAYLOAD_FILE=${PAC_PAYLOAD_FILE}

go run cmd/pipelines-as-code/main.go
