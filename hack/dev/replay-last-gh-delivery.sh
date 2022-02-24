#!/usr/bin/env bash
# Chmouel Boudjnah <chmouel@chmouel.com>
set -euf

ROOTDIR=$(git rev-parse --show-toplevel)
cd ${ROOTDIR}
jtoken=$(./hack/dev/gen-token.py)

TMP=$(mktemp /tmp/.mm.XXXXXX)
clean() { rm -f ${TMP}; }
trap clean EXIT

TARGET=${1:-""}

[[ -z ${TARGET} ]] && {
    echo "need a target url"
    exit 1
}

sleep 2
EVENT_SHIFT=${2:-0}
delivery_id=$(http https://api.github.com/app/hook/deliveries "Authorization: Bearer ${jtoken}"  | jq ".[0].id")
http https://api.github.com/app/hook/deliveries/${delivery_id} "Authorization: Bearer ${jtoken}" > ${TMP}

headers=$(jq .request.headers ${TMP}|python -c 'import sys, json;dico=json.load(sys.stdin);[ print(f"{k}:{dico[k]} ",end="") for k in dico];print()')

jq .request.headers ${TMP}|python -c 'import sys, json;dico=json.load(sys.stdin);[ print(f"{k}:{dico[k]} ") for k in dico];print()' > /tmp/last-event-payload.headers
jq -r .request.payload ${TMP} > /tmp/last-event-payload.json

jq -r .request.payload ${TMP}|http POST ${TARGET} ${headers}
