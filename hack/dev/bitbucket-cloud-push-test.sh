#!/usr/bin/env bash
# Chmouel Boudjnah <chmouel@chmouel.com>
# 
# Run pipelines-as-code locally for bitbucket target
#
# Set a payload for a target profile and go run pipelines as code with it.
#
# You have three profile :
# 
#   owner - owner of the repo
#   member - member of the workspace
#   other - a non member
# 
# each submit a PR and have a PR number which maps into the ROLE_TO_PR variable
# and from the API we get all the information needed.
#
# env variable to set in envrc :
#
# export TEST_BITBUCKET_CLOUD_USER=
# export TEST_BITBUCKET_CLOUD_API_URL=
# export TEST_BITBUCKET_CLOUD_TEST_REPOSITORY=
# export TEST_BITBUCKET_CLOUD_OTHER_USER=
# export TEST_BITBUCKET_CLOUD_OTHER_TOKEN=
# export TEST_BITBUCKET_CLOUD_MEMBER_USER=
# export TEST_BITBUCKET_CLOUD_MEMBER_TOKEN=

set -eux

[[ -z ${TEST_BITBUCKET_CLOUD_TOKEN:-""} ]] && { echo "We need the TEST_BITBUCKET_CLOUD_TOKEN variable set"; exit 1 ;}
[[ -z ${TEST_BITBUCKET_CLOUD_USER:-""} ]] && { echo "We need the TEST_BITBUCKET_CLOUD_USER variable set"; exit 1 ;}
[[ -z ${TEST_BITBUCKET_CLOUD_API_URL:-""} ]] && { echo "We need the TEST_BITBUCKET_CLOUD_API_URL variable set"; exit 1 ;}
[[ -z ${TEST_BITBUCKET_CLOUD_TEST_REPOSITORY:-""} ]] && { echo "We need the TEST_BITBUCKET_CLOUD_TEST_REPOSITORY variable set"; exit 1 ;}
type -p http >/dev/null 2>/dev/null || { echo "We need httpie installed https://httpie.io/docs#installation"; exit 1 ;}

ROOTDIR=$(git rev-parse --show-toplevel)
OWNER=${TEST_BITBUCKET_CLOUD_TEST_REPOSITORY%/*}
REPOSITORY=${TEST_BITBUCKET_CLOUD_TEST_REPOSITORY#*/}
WEBHOOK_TYPE=push
TRIGGER_TARGET=push
TMP=$(mktemp /tmp/.mm.XXXXXX)

export PAC_GIT_PROVIDER_TYPE="bitbucket-cloud"
export PAC_SECRET_AUTO_CREATE=true
export PAC_BITBUCKET_CLOUD_CHECK_SOURCE_IP=true
export PAC_SOURCE_IP="127.0.0.1,18.246.31.224"

clean() { rm -vf "${TMP}" ${TMP}*.json ;}
trap clean EXIT

get_info() {
    local apipath=${1}
    local user=${2}
    local token=${3}
    local typeof=${4}    

    local tmpfile=${TMP}-$(echo ${apipath}|tr -d /).json
    
    [[ ! -s ${tmpfile} ]] &&
        http -f -q --no-verbose --check-status --auth "${user}:${token}" GET ${TEST_BITBUCKET_CLOUD_API_URL}${apipath} \
         -d --output="${tmpfile}"
    jq -r ${typeof} ${tmpfile}    
}



function generate() {
    ACCOUNT_ID=$(get_info /user "${TEST_BITBUCKET_CLOUD_USER}" ${TEST_BITBUCKET_CLOUD_TOKEN} .account_id)
    NICKNAME=$(get_info /user "${TEST_BITBUCKET_CLOUD_USER}" ${TEST_BITBUCKET_CLOUD_TOKEN} .nickname)
    MAINBRANCH=$(get_info /repositories/${TEST_BITBUCKET_CLOUD_TEST_REPOSITORY} \
                          "${TEST_BITBUCKET_CLOUD_USER}" ${TEST_BITBUCKET_CLOUD_TOKEN} .mainbranch.name)
    TARGETHASH=$(get_info /repositories/${TEST_BITBUCKET_CLOUD_TEST_REPOSITORY}/commits "${TEST_BITBUCKET_CLOUD_USER}" ${TEST_BITBUCKET_CLOUD_TOKEN} .values[0].hash)    
    cat <<EOF> ${PAYLOAD_FILE}
{
    "repository": {
        "workspace": {
            "slug": "${OWNER}"
        },
        "name": "${REPOSITORY}",
        "links": {
            "html": {
                "href": "https://bitbucket.org/${TEST_BITBUCKET_CLOUD_TEST_REPOSITORY}"
            }
        }
    },
    "actor": {
        "account_id": "${ACCOUNT_ID}",
        "nickname": "${NICKNAME}"
    },
    "push": {
        "changes": [{
            "new": {
                "name": "${MAINBRANCH}",
                "target": {
                    "hash": "${TARGETHASH}"
                }
            },
            "old": {
                "name": "${MAINBRANCH}"
            }
        }]
    }
}
EOF
}

[[ -n ${PAYLOAD_FILE:-""} && -f ${PAYLOAD_FILE} ]] || {
    PAYLOAD_FILE=/tmp/payload-push-${OWNER}-${REPOSITORY}.json
    generate
    cat ${PAYLOAD_FILE}
}
 
cd ${ROOTDIR}

go run cmd/pipelines-as-code/main.go run \
      --payload-file="${PAYLOAD_FILE}"  --webhook-type="${WEBHOOK_TYPE}" \
      --trigger-target="${TRIGGER_TARGET}"
