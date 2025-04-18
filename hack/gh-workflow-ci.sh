#!/usr/bin/env bash
# shellcheck disable=SC2038,SC2153
# Helper script for GitHub Actions CI, used from e2e tests.
set -exufo pipefail

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
  make test-e2e GO_TEST_FLAGS="-run \"$(echo "${tests[*]}" | sed 's/ /|/g')\""
}

startpaac() {
  echo "**********************************************************************"
  echo "                       Installing startpaac"
  echo "**********************************************************************"
  [[ -d ~/startpaac ]] ||
    git clone --depth=1 https://github.com/chmouel/startpaac ~/startpaac

  mkdir -p ~/second ~/pass $HOME/.config/startpaac

  cat <<EOF >$HOME/.config/startpaac/config
PAC_DIR=$HOME/work/pipelines-as-code/pipelines-as-code/
PAC_SECRET_FOLDER=$HOME/pass
PAC_SECOND_SECRET_FOLDER=${HOME}/second
TARGET_HOST=local
EOF

  echo "${PAC_GITHUB_PRIVATE_KEY}" >~/pass/github-private-key
  echo "${PAC_GITHUB_APPLICATION_ID}" >~/pass/github-application-id
  echo "${PAC_WEBHOOK_SECRET}" >~/pass/webhook.secret
  echo "${PAC_SMEE_URL}" >~/pass/smee

  echo "${TEST_GITHUB_SECOND_PRIVATE_KEY}" >~/second/github-private-key
  echo "${TEST_GITHUB_SECOND_APPLICATION_ID}" >~/second/github-application-id
  echo "${TEST_GITHUB_SECOND_WEBHOOK_SECRET}" >~/second/webhook.secret
  echo "${TEST_GITHUB_SECOND_SMEE_URL}" >~/second/smee

  go install github.com/jsha/minica@latest

  (
    cd ${HOME}/startpaac
    if [[ ${TEST_PROVIDER} == "providers" ]]; then
      ./startpaac --all-github-second-no-forgejo
    else
      ./startpaac --all
    fi
  )

  echo "**********************************************************************"
  echo "Copying minica CA certs to /usr/local/share/ca-certificates/minica.crt"
  echo "**********************************************************************"
  sudo cp -v /tmp/certs/minica.pem /usr/local/share/ca-certificates/minica.crt
  sudo update-ca-certificates
}

collect_logs() {
  # Read from environment variables
  local test_gitea_smee_url="${TEST_GITEA_SMEEURL}"
  local github_ghe_smee_url="${TEST_GITHUB_SECOND_SMEE_URL}"

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

  startpaac
    Install startpaac and setup the config
    Required env vars: PAC_GITHUB_PRIVATE_KEY, PAC_GITHUB_APPLICATION_ID, PAC_WEBHOOK_SECRET, PAC_SMEE_URL
EOF
}

case ${1-""} in
run_e2e_tests)
  run_e2e_tests
  ;;
collect_logs)
  collect_logs
  ;;
startpaac)
  startpaac
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
