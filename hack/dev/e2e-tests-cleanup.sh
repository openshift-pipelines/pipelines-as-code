#!/usr/bin/env bash
set -eu

type -p gh >/dev/null || { echo "You need gh installed and configured for ${TEST_GITHUB_API_URL}"; exit 1 ;}

export GH_REPO=${TEST_GITHUB_REPO_OWNER}
export GH_HOST=$(echo ${TEST_GITHUB_API_URL}|sed 's,https://,,')

echo "Closing lingering PR on ${TEST_GITHUB_API_URL}"
for prn in $(gh pr list --jq .[].number --json number);do
    gh pr close ${prn}
done

echo "Cleaning Namespaces"
{ kubectl get ns -o name|grep pac-e2e|sed 's/namespace.//'|xargs -r -P5 kubectl delete ns ;}

