#!/usr/bin/env bash
# Replay a pac pipelinerun grabbing its payload and env from the output
# need tkn, fzf
#
# It will save the payload in the /tmp/pac-last-run directory so we can replay them at will.
# There is even a /tmp/pac-last-run/run.sh to replay things as you want.
#
# This generate files to replay easily in vscode/lsp (intelij/goland is annoying
# since doesn't support envFile)
#
# This is my launch configuration
# {
#    "name": "PAC last",
#    "type": "go",
#    "request": "launch",
#    "cwd": "${workspaceFolder}",
#    "program": "${workspaceFolder}/cmd/pipelines-as-code",
#    "envFile": "/tmp/pac-last-run/env",
#    "env": {
#      "KUBECONFIG": "${env:HOME}/.kube/config",
#    }
#  },

set -euf

TMPD=/tmp/pac-last-run
rm -rf ${TMPD};mkdir -p ${TMPD}

if [[ ${1:-""} == -l ]];then
    arg="-L"
elif [[ -n ${1:-""} ]];then
    arg="${1}"
else
   pr=$(tkn tr ls --no-headers|fzf  -1 --preview 'tkn tr describe `echo {}|sed "s/ .*//"`')
   [[ -z ${pr} ]] && exit 0
   arg=$(echo ${pr}|awk '{print $1}')
   echo "Selected ${arg}"
fi
tkn tr logs ${arg} --prefix=false 2>/dev/null > ${TMPD}/last
[[ -s ${TMPD}/last ]] || { echo "payload could not be found"; exit 1 ;}

export PAC_PAYLOAD_FILE=${TMPD}/payload.json
sed '/^PAC_/,$ { d;}' ${TMPD}/last > ${PAC_PAYLOAD_FILE}
[[ -s ${TMPD}/payload.json ]] || { echo "payload json could not be found"; exit 1 ;}

grep -e "^PAC_.[a-zA-Z0-9_-]*=" ${TMPD}/last|sed -e 's/=\(.*\)/="\1"/' -e 's/^/export /' > ${TMPD}/env
[[ -s ${TMPD}/env ]] || { echo "payload env could not be found"; exit 1 ;}

sed -i "s,PAC_PAYLOAD_FILE=.*,PAC_PAYLOAD_FILE=${TMPD}/payload.json," $TMPD/env
source $TMPD/env

if [[ -n ${PAC_WORKSPACE_SECRET} ]];then
    for key in github-application-id github-private-key webhook.secret;do
        kubectl get secrets -n pipelines-as-code pipelines-as-code-secret -o json | jq -r ".data.\"${key}\" | @base64d" > ${TMPD}/${key}
    done
    export PAC_WORKSPACE_SECRET=${TMPD}
    sed -i "s,PAC_WORKSPACE_SECRET=.*,PAC_WORKSPACE_SECRET=$TMPD," ${TMPD}/env
fi

cat ${TMPD}/env > ${TMPD}/run.sh
echo "go run cmd/pipelines-as-code/main.go" >> ${TMPD}/run.sh
chmod +x ${TMPD}/run.sh


sed -i 's/^export //' ${TMPD}/env
cat ${TMPD}/env

go run cmd/pipelines-as-code/main.go
