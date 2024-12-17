#!/usr/bin/env bash
# shellcheck disable=SC2038
# Helper script for GitHub Actions CI, used from e2e tests.
set -exufo pipefail

create_pac_github_app_secret() {
  local app_private_key="${1}"
  local application_id="${2}"
  local webhook_secret="${3}"
  kubectl delete secret -n pipelines-as-code pipelines-as-code-secret || true
  kubectl -n pipelines-as-code create secret generic pipelines-as-code-secret \
    --from-literal github-private-key="${app_private_key}" \
    --from-literal github-application-id="${application_id}" \
    --from-literal webhook.secret="${webhook_secret}"
  kubectl patch configmap -n pipelines-as-code -p "{\"data\":{\"bitbucket-cloud-check-source-ip\": \"false\"}}" \
    --type merge pipelines-as-code

  # restart controller
  kubectl -n pipelines-as-code delete pod -l app.kubernetes.io/name=controller

  echo -n "Waiting for controller to restart"
  i=0
  while true; do
    [[ ${i} == 120 ]] && exit 1
    ep=$(kubectl get ep -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.subsets[*].addresses[*].ip}')
    [[ -n ${ep} ]] && break
    sleep 2
    echo -n "."
    i=$((i + 1))
  done
  echo
}

create_second_github_app_controller_on_ghe() {
  local test_github_second_smee_url="${1}"
  local test_github_second_private_key="${2}"
  local test_github_second_webhook_secret="${3}"

  if [[ -n "$(type -p apt)" ]]; then
    apt update &&
      apt install -y python3-yaml
  elif [[ -n "$(type -p dnf)" ]]; then
    dnf install -y python3-pyyaml
  else
    # TODO(chmouel): setup a virtualenvironment instead
    python3 -m pip install --break-system-packages PyYAML
  fi

  ./hack/second-controller.py \
    --controller-image="ko" \
    --smee-url="${test_github_second_smee_url}" \
    --ingress-domain="paac-127-0-0-1.nip.io" \
    --namespace="pipelines-as-code" \
    ghe | tee /tmp/generated.yaml

  ko apply -f /tmp/generated.yaml
  kubectl delete secret -n pipelines-as-code ghe-secret || true
  kubectl -n pipelines-as-code create secret generic ghe-secret \
    --from-literal github-private-key="${test_github_second_private_key}" \
    --from-literal github-application-id="2" \
    --from-literal webhook.secret="${test_github_second_webhook_secret}"
  sed "s/name: pipelines-as-code/name: ghe-configmap/" <config/302-pac-configmap.yaml | kubectl apply -n pipelines-as-code -f-
  kubectl patch configmap -n pipelines-as-code ghe-configmap -p '{"data":{"application-name": "Pipelines as Code GHE"}}'
  kubectl -n pipelines-as-code delete pod -l app.kubernetes.io/name=ghe-controller
}

run_e2e_tests() {
  bitbucket_cloud_token="${1}"
  webhook_secret="${2}"
  test_gitea_smeeurl="${3}"
  installation_id="${4}"
  gh_apps_token="${5}"
  test_github_second_token="${6}"
  gitlab_token="${7}"
  bitbucket_server_token="${8}"
  bitbucket_server_api_url="${9}"
  bitbucket_server_webhook_secret="${10}"

  # Nothing specific to webhook here it  just that repo is private in that org and that's what we want to test
  export TEST_GITHUB_PRIVATE_TASK_URL="https://github.com/openshift-pipelines/pipelines-as-code-e2e-tests-private/blob/main/remote_task.yaml"
  export TEST_GITHUB_PRIVATE_TASK_NAME="task-remote"

  export GO_TEST_FLAGS="-v -race -failfast"

  export TEST_BITBUCKET_CLOUD_API_URL=https://api.bitbucket.org/2.0
  export TEST_BITBUCKET_CLOUD_E2E_REPOSITORY=cboudjna/pac-e2e-tests
  export TEST_BITBUCKET_CLOUD_TOKEN=${bitbucket_cloud_token}
  export TEST_BITBUCKET_CLOUD_USER=cboudjna

  export TEST_EL_URL="http://${CONTROLLER_DOMAIN_URL}"
  export TEST_EL_WEBHOOK_SECRET="${webhook_secret}"

  export TEST_GITEA_API_URL="http://localhost:3000"
  ## This is the URL used to forward requests from the webhook to the paac controller
  ## badly named!
  export TEST_GITEA_SMEEURL="${test_gitea_smeeurl}"
  export TEST_GITEA_USERNAME=pac
  export TEST_GITEA_PASSWORD=pac
  export TEST_GITEA_REPO_OWNER=pac/pac

  export TEST_GITHUB_API_URL=api.github.com
  export TEST_GITHUB_REPO_INSTALLATION_ID="${installation_id}"
  export TEST_GITHUB_REPO_OWNER_GITHUBAPP=openshift-pipelines/pipelines-as-code-e2e-tests
  export TEST_GITHUB_REPO_OWNER_WEBHOOK=openshift-pipelines/pipelines-as-code-e2e-tests-webhook
  export TEST_GITHUB_TOKEN="${gh_apps_token}"

  export TEST_GITHUB_SECOND_API_URL=ghe.pipelinesascode.com
  export TEST_GITHUB_SECOND_EL_URL=http://ghe.paac-127-0-0-1.nip.io
  export TEST_GITHUB_SECOND_REPO_OWNER_GITHUBAPP=pipelines-as-code/e2e
  # TODO: webhook repo for second github
  # export TEST_GITHUB_SECOND_REPO_OWNER_WEBHOOK=openshift-pipelines/pipelines-as-code-e2e-tests-webhook
  export TEST_GITHUB_SECOND_REPO_INSTALLATION_ID=1
  export TEST_GITHUB_SECOND_TOKEN="${test_github_second_token}"

  export TEST_GITLAB_API_URL="https://gitlab.com"
  export TEST_GITLAB_PROJECT_ID="34405323"
  export TEST_GITLAB_TOKEN=${gitlab_token}
  # https://gitlab.com/gitlab-com/alliances/ibm-red-hat/sandbox/openshift-pipelines/pac-e2e-tests

  export TEST_BITBUCKET_SERVER_TOKEN="${bitbucket_server_token}"
  export TEST_BITBUCKET_SERVER_API_URL="${bitbucket_server_api_url}"
  export TEST_BITBUCKET_SERVER_WEBHOOK_SECRET="${bitbucket_server_webhook_secret}"
  export TEST_BITBUCKET_SERVER_USER="pipelines"
  export TEST_BITBUCKET_SERVER_E2E_REPOSITORY="PAC/pac-e2e-tests"
  make test-e2e
}

collect_logs() {
  test_gitea_smee_url="${1}"
  github_ghe_smee_url="${2}"
  mkdir -p /tmp/logs
  kind export logs /tmp/logs
  [[ -d /tmp/gosmee-replay ]] && cp -a /tmp/gosmee-replay /tmp/logs/

  kubectl get pipelineruns -A -o yaml >/tmp/logs/pac-pipelineruns.yaml
  kubectl get repositories.pipelinesascode.tekton.dev -A -o yaml >/tmp/logs/pac-repositories.yaml
  kubectl get configmap -n pipelines-as-code -o yaml >/tmp/logs/pac-configmap
  kubectl get events -A >/tmp/logs/events

  allNamespaces=$(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}')
  for ns in ${allNamespaces}; do
    mkdir -p /tmp/logs/ns/${ns}
    for type in pods pipelineruns repositories configmap; do
      kubectl get ${type} -n ${ns} -o yaml >/tmp/logs/ns/${ns}/${type}.yaml
    done
    kubectl -n ${ns} get events >/tmp/logs/ns/${ns}/events
  done

  for url in "${test_gitea_smee_url}" "${github_ghe_smee_url}"; do
    find /tmp/logs -type f -exec grep -l "${url}" {} \; | xargs -r sed -i "s|${url}|SMEE_URL|g"
  done

  detect_panic
}

detect_panic() {
  # shellcheck disable=SC2016
  (find /tmp/logs/ -type f -regex '.*/pipelines-as-code.*/[0-9]\.log$' | xargs -r sed -n '/stderr F panic:.*/,$p') >/tmp/panic.log
  if [[ -s /tmp/panic.log ]]; then
    set +x
    echo "=====================  PANIC DETECTED ====================="
    echo "**********************************************************************"
    cat /tmp/panic.log
    echo "**********************************************************************"
    exit 1
  fi
}

help() {
  cat <<EOF
  Usage: $0 <command> [args]

  Shell script to run e2e tests from GitHub Actions CI

  create_pac_github_app_secret <application_id> <app_private_key> <webhook_secret>
    Create the secret for the github app

  create_second_github_app_controller_on_ghe <test_github_second_smee_url> <test_github_second_private_key> <test_github_second_webhook_secret>
    Create the second controller on GHE

  run_e2e_tests <bitbucket_cloud_token> <webhook_secret> <test_gitea_smeeurl> <installation_id> <gh_apps_token> <test_github_second_token> <gitlab_token> <bitbucket_server_token> <bitbucket_server_api_url> <bitbucket_server_webhook_secret>
    Run the e2e tests

  collect_logs
    Collect logs from the cluster
EOF
}

case ${1-""} in
create_pac_github_app_secret)
  create_pac_github_app_secret "${2}" "${3}" "${4}"
  ;;
create_second_github_app_controller_on_ghe)
  create_second_github_app_controller_on_ghe "${2}" "${3}" "${4}"
  ;;
run_e2e_tests)
  run_e2e_tests "${2}" "${3}" "${4}" "${5}" "${6}" "${7}" "${8}" "${9}" "${10}" "${11}"
  ;;
collect_logs)
  collect_logs "${2}" "${3}"
  ;;
help)
  help
  exit 0
  ;;
*)
  echo "Unknown command ${1-}"
  help
  exit 1
  ;;
esac
