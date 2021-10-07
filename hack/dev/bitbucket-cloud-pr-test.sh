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
# export TEST_BITBUCKET_USER=
# export TEST_BITBUCKET_API_URL=
# export TEST_BITBUCKET_TEST_REPOSITORY=
# export TEST_BITBUCKET_OTHER_USER=
# export TEST_BITBUCKET_OTHER_TOKEN=
# export TEST_BITBUCKET_MEMBER_USER=
# export TEST_BITBUCKET_MEMBER_TOKEN=

set -eux

PROFILE=${1:-owner}
# Which role submitted which PR number
declare -A ROLE_TO_PR=(
    [owner]=3
    [member]=4
    [other]=5
)

[[ -z ${TEST_BITBUCKET_TOKEN:-""} ]] && { echo "We need the TEST_BITBUCKET_TOKEN variable set"; exit 1 ;}
[[ -z ${TEST_BITBUCKET_USER:-""} ]] && { echo "We need the TEST_BITBUCKET_USER variable set"; exit 1 ;}
[[ -z ${TEST_BITBUCKET_API_URL:-""} ]] && { echo "We need the TEST_BITBUCKET_API_URL variable set"; exit 1 ;}
[[ -z ${TEST_BITBUCKET_TEST_REPOSITORY:-""} ]] && { echo "We need the TEST_BITBUCKET_TEST_REPOSITORY variable set"; exit 1 ;}
type -p http >/dev/null 2>/dev/null || { echo "We need httpie installed https://httpie.io/docs#installation"; exit 1 ;}

ROOTDIR=$(git rev-parse --show-toplevel)
OWNER=${TEST_BITBUCKET_TEST_REPOSITORY%/*}
REPOSITORY=${TEST_BITBUCKET_TEST_REPOSITORY#*/}
WEBHOOK_TYPE=pull_request
TRIGGER_TARGET=pull_request
TMP=$(mktemp /tmp/.mm.XXXXXX)
PAYLOAD_FILE=/tmp/payload-pullrequest-${OWNER}-${REPOSITORY}.json

export PAC_WEBVCS_TYPE="bitbucket-cloud"
export PAC_SECRET_AUTO_CREATE=true
export PAC_BITBUCKET_CLOUD_CHECK_SOURCE_IP=true
export PAC_SOURCE_IP="1.2.3.4,127.0.0.1"
export PAC_BITBUCKET_CLOUD_ADDITIONAL_SOURCE_IP="127.0.0.1"

clean() { rm -f "${TMP}" ;}
trap clean EXIT

get_user_info() {
    local user=${1}
    local token=${2}
    local typeof=${3}

    [[ ! -s ${TMP} ]] && \
        http -q --no-verbose --check-status --auth "${user}:${token}" GET ${TEST_BITBUCKET_API_URL}/user -d --output="${TMP}"
    jq -r ${typeof} ${TMP}
}

get_pr_info() {
    http -q --no-verbose --check-status --auth "${TEST_BITBUCKET_USER}:${TEST_BITBUCKET_TOKEN}" GET \
         ${TEST_BITBUCKET_API_URL}/repositories/${TEST_BITBUCKET_TEST_REPOSITORY}/pullrequests/${PULL_REQUEST_ID} \
         -d --output="${TMP}"
    HASH=$(jq -r '.source.commit.hash' ${TMP})
    DEFAULT_BRANCH=$(jq -r '.destination.branch.name' ${TMP})
    HEAD_BRANCH=$(jq -r '.source.branch.name' ${TMP})
}

if [[ ${PROFILE} == "other" ]];then
   [[ -z ${TEST_BITBUCKET_OTHER_USER:-""} ]] && { echo "We need the TEST_BITBUCKET_OTHER_USER variable set"; exit 1 ;}
   [[ -z ${TEST_BITBUCKET_OTHER_TOKEN:-""} ]] && { echo "We need the TEST_BITBUCKET_OTHER_TOKEN variable set"; exit 1 ;}
   
   target_user=${TEST_BITBUCKET_OTHER_USER}
   target_token=${TEST_BITBUCKET_OTHER_TOKEN}
elif [[ ${PROFILE} == "member" ]];then
     [[ -z ${TEST_BITBUCKET_MEMBER_USER:-""} ]] && { echo "We need the TEST_BITBUCKET_MEMBER_USER variable set"; exit 1 ;}
     [[ -z ${TEST_BITBUCKET_MEMBER_TOKEN:-""} ]] && { echo "We need the TEST_BITBUCKET_MEMBER_TOKEN variable set"; exit 1 ;}
        
     target_user=${TEST_BITBUCKET_MEMBER_USER}
     target_token=${TEST_BITBUCKET_MEMBER_TOKEN}
elif [[ ${PROFILE} == "owner" ]];then
     target_user=${TEST_BITBUCKET_USER}
     target_token=${TEST_BITBUCKET_TOKEN}
fi
     
ACCOUNT_ID=$(get_user_info "${target_user}" ${target_token} .account_id)
NICKNAME=$(get_user_info "${target_user}" ${target_token} .nickname)
PULL_REQUEST_ID=${ROLE_TO_PR[${PROFILE}]}

get_pr_info

cat << EOF | tee ${PAYLOAD_FILE}
{
    "repository": {
        "workspace": {
            "slug": "${OWNER}"
        },
        "name": "${REPOSITORY}",
        "links": {
            "html": {
                "href": "https://bitbucket.org/${OWNER}/${REPOSITORY}"
            }
        }
    },
    "pullrequest": {
        "id": ${PULL_REQUEST_ID},
        "author": {
            "account_id": "${ACCOUNT_ID}",
            "nickname": "${NICKNAME}"
        },
        "destination": {
            "branch": {
                "name": "${DEFAULT_BRANCH}"
            }
        },
        "source": {
            "branch": {
                "name": "${HEAD_BRANCH}"
            },
            "commit": {
                "hash": "${HASH:0:12}"
            }
        }
    }
}
EOF
cd ${ROOTDIR}

go run cmd/pipelines-as-code/main.go run \
   --payload-file="${PAYLOAD_FILE}"  --webhook-type="${WEBHOOK_TYPE}" \
   --trigger-target="${TRIGGER_TARGET}"
