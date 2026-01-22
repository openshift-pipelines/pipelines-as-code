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
  local test_github_second_application_id="${TEST_GITHUB_SECOND_APPLICATION_ID}"
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

  ko apply --insecure-registry -f /tmp/generated.yaml
  kubectl delete secret -n pipelines-as-code ghe-secret || true
  kubectl -n pipelines-as-code create secret generic ghe-secret \
    --from-literal github-private-key="${test_github_second_private_key}" \
    --from-literal github-application-id="${test_github_second_application_id}" \
    --from-literal webhook.secret="${test_github_second_webhook_secret}"
  sed "s/name: pipelines-as-code/name: ghe-configmap/" <config/302-pac-configmap.yaml | kubectl apply -n pipelines-as-code -f-
  kubectl patch configmap -n pipelines-as-code ghe-configmap -p '{"data":{"application-name": "Pipelines as Code GHE"}}'
  kubectl -n pipelines-as-code delete pod -l app.kubernetes.io/name=ghe-controller
}

get_tests() {
  local target="$1"
  local -a testfiles
  local all_tests
  mapfile -t testfiles < <(find test/ -maxdepth 1 -name '*.go')
  all_tests=$(grep -hioP '^func[[:space:]]+Test[[:alnum:]_]+' "${testfiles[@]}" | sed -E 's/^func[[:space:]]+//')

  local -a gitea_tests
  local chunk_size remainder
  if [[ "${target}" == *"gitea"* ]]; then
    # Filter Gitea tests, excluding Concurrency tests
    mapfile -t gitea_tests < <(echo "${all_tests}" | grep -iP '^TestGitea' 2>/dev/null | grep -ivP 'Concurrency' 2>/dev/null | sort 2>/dev/null)
    # Remove any non-Gitea entries that might have been captured
    local -a filtered_tests
    for test in "${gitea_tests[@]}"; do
      if [[ "${test}" =~ ^TestGitea ]] && [[ ! "${test}" =~ Concurrency ]]; then
        filtered_tests+=("${test}")
      fi
    done
    gitea_tests=("${filtered_tests[@]}")
    chunk_size=$((${#gitea_tests[@]} / 3))
    remainder=$((${#gitea_tests[@]} % 3))
  fi

  case "${target}" in
  concurrency)
    printf '%s\n' "${all_tests}" | grep -iP 'Concurrency'
    ;;
  github)
    printf '%s\n' "${all_tests}" | grep -iP 'Github' | grep -ivP 'Concurrency|GithubSecond'
    ;;
  github_second_controller)
    printf '%s\n' "${all_tests}" | grep -iP 'GithubSecond' | grep -ivP 'Concurrency'
    ;;
  gitlab_bitbucket)
    printf '%s\n' "${all_tests}" | grep -iP 'Gitlab|Bitbucket' | grep -ivP 'Concurrency'
    ;;
  gitea_1)
    if [[ ${#gitea_tests[@]} -gt 0 ]]; then
      printf '%s\n' "${gitea_tests[@]:0:${chunk_size}}"
    fi
    ;;
  gitea_2)
    if [[ ${#gitea_tests[@]} -gt 0 ]]; then
      printf '%s\n' "${gitea_tests[@]:${chunk_size}:${chunk_size}}"
    fi
    ;;
  gitea_3)
    if [[ ${#gitea_tests[@]} -gt 0 ]]; then
      local start_idx=$((chunk_size * 2))
      printf '%s\n' "${gitea_tests[@]:${start_idx}:$((chunk_size + remainder))}"
    fi
    ;;
  gitea_others)
    # Deprecated: Use gitea_1, gitea_2, gitea_3 instead
    printf '%s\n' "${all_tests}" | grep -ivP 'Github|Gitlab|Bitbucket|Concurrency'
    ;;
  *)
    echo "Invalid target: ${target}"
    echo "supported targets: github, github_second_controller, gitlab_bitbucket, gitea_1, gitea_2, gitea_3, concurrency"
    ;;
  esac
}

run_e2e_tests() {
  set +x
  target="${TEST_PROVIDER}"
  export PAC_E2E_KEEP_NS=true
  
  mkdir -p /tmp/logs

  mapfile -t tests < <(get_tests "${target}")
  echo "About to run ${#tests[@]} tests: ${tests[*]}"
  # shellcheck disable=SC2001
  test_pattern="$(echo "${tests[*]}" | sed 's/ /|/g')"

  # Use gotestsum if available for better output and JUnit XML generation
  if command -v gotestsum >/dev/null 2>&1; then
    echo "Using gotestsum for test execution..."
    mkdir -p /tmp/test-results
    env GODEBUG=asynctimerchan=1 gotestsum \
      --junitfile /tmp/test-results/e2e-tests.xml \
      --jsonfile /tmp/test-results/e2e-tests.json \
      --junitfile-testsuite-name short \
      --junitfile-testcase-classname short \
      --format standard-verbose \
      -- \
      -race -failfast -timeout 45m -count=1 -tags=e2e \
      -v -run "${test_pattern}" \
      ./test 2>&1 | tee -a /tmp/logs/e2e-test-output.log
    return ${PIPESTATUS[0]}
  else
    echo "gotestsum not found, using make test-e2e..."
    make test-e2e GO_TEST_FLAGS="-v -run \"${test_pattern}\"" 2>&1 | tee -a /tmp/logs/e2e-test-output.log
    return ${PIPESTATUS[0]}
  fi
}

output_logs() {
  if command -v "snazy" >/dev/null 2>&1; then
    snazy --extra-fields --skip-line-regexp="^(Reconcile (succeeded|error)|Updating webhook)" -f error -f fatal /tmp/logs/pac-pods.log
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

  # Collect test results from gotestsum (JUnit XML and JSON)
  if [[ -d /tmp/test-results ]]; then
    cp -a /tmp/test-results /tmp/logs/test-results
    echo "Copied test results to /tmp/logs/test-results"
  fi

  # Collect all gosmee data in organized directory
  mkdir -p /tmp/logs/gosmee
  [[ -d /tmp/gosmee-replay ]] && cp -a /tmp/gosmee-replay /tmp/logs/gosmee/replay
  [[ -d /tmp/gosmee-replay-ghe ]] && cp -a /tmp/gosmee-replay-ghe /tmp/logs/gosmee/replay-ghe
  [[ -f /tmp/gosmee-main.log ]] && cp /tmp/gosmee-main.log /tmp/logs/gosmee/main.log
  [[ -f /tmp/gosmee-ghe.log ]] && cp /tmp/gosmee-ghe.log /tmp/logs/gosmee/ghe.log

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

notify_slack() {
  # Required env vars: SLACK_WEBHOOK_URL, GITHUB_REPOSITORY, GITHUB_REF_NAME, GITHUB_SERVER_URL, GITHUB_RUN_ID, GITHUB_SHA
  # Required argument: artifacts directory path
  local artifacts_dir="${1:-artifacts}"
  local slack_webhook_url="${SLACK_WEBHOOK_URL}"

  if [[ -z "${slack_webhook_url}" ]]; then
    echo "SLACK_WEBHOOK_URL is not set, skipping Slack notification"
    return 0
  fi

  if [[ ! -d "${artifacts_dir}" ]]; then
    echo "Artifacts directory '${artifacts_dir}' not found"
    return 1
  fi

  echo "Scanning artifacts in: ${artifacts_dir}"

  local failure_details=""
  local failed_providers=""

  # Use find to get provider directories (more reliable than glob)
  local provider_dirs
  provider_dirs=$(find "${artifacts_dir}" -maxdepth 1 -type d -name 'logs-e2e-tests-*' | sort)

  if [[ -z "${provider_dirs}" ]]; then
    echo "No provider artifact directories found matching pattern: ${artifacts_dir}/logs-e2e-tests-*"
    return 0
  fi

  echo "Found provider directories:"
  echo "${provider_dirs}"

  while IFS= read -r provider_dir; do
    local provider
    provider=$(basename "${provider_dir}" | sed 's/logs-e2e-tests-//')
    echo "Processing provider: ${provider} (${provider_dir})"

    # Extract failed test names from e2e test output log
    local failed_tests=""
    if [[ -f "${provider_dir}/e2e-test-output.log" ]]; then
      echo "  Found e2e-test-output.log"
      failed_tests=$(grep -E "^--- FAIL:" "${provider_dir}/e2e-test-output.log" 2>/dev/null | sed 's/--- FAIL: //' | cut -d' ' -f1 | sort -u | paste -sd ',' - || true)
      if [[ -n "${failed_tests}" ]]; then
        echo "  Failed tests: ${failed_tests}"
      else
        echo "  No failed tests found in log"
      fi
    else
      echo "  No e2e-test-output.log found"
      ls -la "${provider_dir}" 2>/dev/null || true
    fi

    # Check if this provider had failures
    if [[ -n "${failed_tests}" ]]; then
      failed_providers="${failed_providers}${provider}, "
      # Use literal \n for Slack mrkdwn newlines (will be kept as-is in JSON)
      failure_details="${failure_details}• *${provider}*: ${failed_tests}\\n"
    fi
  done <<<"${provider_dirs}"

  # Remove trailing comma and space
  failed_providers="${failed_providers%, }"

  if [[ -z "${failure_details}" ]]; then
    echo "No failures detected, skipping Slack notification"
    return 0
  fi

  # Remove trailing \n
  failure_details="${failure_details%\\n}"

  local run_url="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
  local commit_url="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/commit/${GITHUB_SHA}"
  local short_sha="${GITHUB_SHA:0:7}"

  # Build Slack message payload using jq to ensure proper JSON escaping
  local payload
  payload=$(jq -n \
    --arg repo "${GITHUB_REPOSITORY:-unknown}" \
    --arg branch "${GITHUB_REF_NAME:-unknown}" \
    --arg run_url "${run_url}" \
    --arg commit_url "${commit_url}" \
    --arg short_sha "${short_sha:-unknown}" \
    --arg failed_providers "${failed_providers}" \
    --arg failure_details "${failure_details}" \
    '{
      "blocks": [
        {
          "type": "header",
          "text": {
            "type": "plain_text",
            "text": "🔴 E2E Test Failures",
            "emoji": true
          }
        },
        {
          "type": "section",
          "text": {
            "type": "mrkdwn",
            "text": ("*Repository:* " + $repo + "\n*Branch:* " + $branch + "\n*Commit:* <" + $commit_url + "|" + $short_sha + ">\n*Workflow:* <" + $run_url + "|View Run>")
          }
        },
        {
          "type": "divider"
        },
        {
          "type": "section",
          "text": {
            "type": "mrkdwn",
            "text": ("*Failed Providers:* " + $failed_providers)
          }
        },
        {
          "type": "section",
          "text": {
            "type": "mrkdwn",
            "text": ("*Failure Details:*\n" + $failure_details)
          }
        }
      ]
    }')

  echo "Sending Slack notification for failed providers: ${failed_providers}"
  curl -s -X POST -H 'Content-type: application/json' --data "${payload}" "${slack_webhook_url}"
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

  notify_slack <artifacts_dir>
    Send a combined Slack notification for all failed E2E test providers.
    Parses test output logs from artifacts to extract failed test names.
    Required env vars: SLACK_WEBHOOK_URL, GITHUB_REPOSITORY, GITHUB_REF_NAME, GITHUB_SERVER_URL, GITHUB_RUN_ID
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
notify_slack)
  notify_slack "${2:-artifacts}"
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
