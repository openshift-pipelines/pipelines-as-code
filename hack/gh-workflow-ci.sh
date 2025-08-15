#!/usr/bin/env bash
# shellcheck disable=SC2038,SC2153
# Helper script for GitHub Actions CI, used from e2e tests.
set -exufo pipefail

export PAC_API_INSTRUMENTATION_DIR=/tmp/api-instrumentation

create_pac_github_app_secret() {
  # Read from environment variables instead of arguments
  local app_private_key="${PAC_GITHUB_PRIVATE_KEY}"
  local application_id="${PAC_GITHUB_APPLICATION_ID}"
  local webhook_secret="${PAC_WEBHOOK_SECRET}"

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
  # Read from environment variables instead of arguments
  local test_github_second_smee_url="${TEST_GITHUB_SECOND_SMEE_URL}"
  local test_github_second_private_key="${TEST_GITHUB_SECOND_PRIVATE_KEY}"
  local test_github_second_webhook_secret="${TEST_GITHUB_SECOND_WEBHOOK_SECRET}"

  if [[ -n "$(type -p apt)" ]]; then
    sudo apt update &&
      sudo apt install -y python3-yaml
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

get_tests() {
  target=$1
  mapfile -t testfiles < <(find test/ -maxdepth 1 -name '*.go')
  ghglabre="Github|Gitlab|Bitbucket"
  if [[ ${target} == "providers" ]]; then
    grep -hioP "^func Test.*(${ghglabre})(\w+)\(" "${testfiles[@]}" | sed -e 's/func[ ]*//' -e 's/($//'
    elif [[ ${target} == "gitea_others" ]]; then
    grep -hioP '^func Test(\w+)\(' "${testfiles[@]}" | grep -iPv "(${ghglabre})" | sed -e 's/func[ ]*//' -e 's/($//'
  else
    echo "Invalid target: ${target}"
    echo "supported targets: githubgitlab, others"
  fi
}

run_e2e_tests() {
  set +x
  target="${TEST_PROVIDER}"

  mapfile -t tests < <(get_tests "${target}")
  echo "About to run ${#tests[@]} tests: ${tests[*]}"
  # shellcheck disable=SC2001
  make test-e2e GO_TEST_FLAGS="-v -run \"$(echo "${tests[*]}" | sed 's/ /|/g')\""
}

output_logs() {
  if command -v "snazy" >/dev/null 2>&1; then
    snazy --extra-fields --skip-line-regexp="^(Reconcile (succeeded|error)|Updating webhook)" /tmp/logs/pac-pods.log
  else
    # snazy for the poors
    python -c "import sys,json,datetime; [print(f'• { (lambda t: datetime.datetime.fromisoformat(t.rstrip(\"Z\")).strftime(\"%H:%M:%S\") if isinstance(t,str) else datetime.datetime.fromtimestamp(t).strftime(\"%H:%M:%S\"))(json.loads(l.strip())[\"ts\"] )} {json.loads(l.strip()).get(\"msg\",\"\")}') if l.strip().startswith('{') else print(l.strip()) for l in sys.stdin]" \
      </tmp/logs/pac-pods.log
  fi
}

collect_logs() {
  # Read from environment variables
  local test_gitea_smee_url="${TEST_GITEA_SMEEURL}"
  local github_ghe_smee_url="${TEST_GITHUB_SECOND_SMEE_URL}"

  mkdir -p /tmp/logs
  # Output logs to stdout so we can see via the web interface directly
  kubectl logs -n pipelines-as-code -l app.kubernetes.io/part-of=pipelines-as-code \
    --all-containers=true --tail=1000 >/tmp/logs/pac-pods.log
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

  if [[ -d ${PAC_API_INSTRUMENTATION_DIR} && -n "$(ls -A ${PAC_API_INSTRUMENTATION_DIR})" ]]; then
    echo "Copying API instrumentation logs from ${PAC_API_INSTRUMENTATION_DIR}"
    cp -a ${PAC_API_INSTRUMENTATION_DIR} /tmp/logs/$(basename ${PAC_API_INSTRUMENTATION_DIR})
  fi

  for url in "${test_gitea_smee_url}" "${github_ghe_smee_url}"; do
    find /tmp/logs -type f -exec grep -l "${url}" {} \; | xargs -r sed -i "s|${url}|SMEE_URL|g"
  done

  detect_panic
}

detect_panic() {
  # shellcheck disable=SC2016
  (find /tmp/logs/ -type f -regex '.*/pipelines-as-code.*/[0-9]\.log$' | xargs -r sed -n '/stderr F panic:.*/,$p' | head -n 80) >/tmp/panic.log
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
  Usage: $0 <command>

  Shell script to run e2e tests from GitHub Actions CI

  Required environment variables depend on the command being executed.

  create_pac_github_app_secret
    Create the secret for the github app
    Required env vars: PAC_GITHUB_PRIVATE_KEY, PAC_GITHUB_APPLICATION_ID, PAC_WEBHOOK_SECRET

  create_second_github_app_controller_on_ghe
    Create the second controller on GHE
    Required env vars: TEST_GITHUB_SECOND_SMEE_URL, TEST_GITHUB_SECOND_PRIVATE_KEY, TEST_GITHUB_SECOND_WEBHOOK_SECRET

  run_e2e_tests
    Run the e2e tests
    Required env vars: TEST_PROVIDER plus many test-specific environment variables

  collect_logs
    Collect logs from the cluster
    Required env vars: TEST_GITEA_SMEEURL, TEST_GITHUB_SECOND_SMEE_URL

  output_logs
    Will output logs using snazzy formatting when available or otherwise through a simple
    python formatter. This makes debugging easier from the GitHub Actions interface.
EOF
}

case ${1-""} in
create_pac_github_app_secret)
  create_pac_github_app_secret
  ;;
create_second_github_app_controller_on_ghe)
  create_second_github_app_controller_on_ghe
  ;;
run_e2e_tests)
  run_e2e_tests
  ;;
collect_logs)
  collect_logs
  ;;
output_logs)
  output_logs
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
