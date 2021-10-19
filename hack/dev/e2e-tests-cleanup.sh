#!/usr/bin/env bash
set -eu

type -p gh >/dev/null || { echo "You need gh installed"; exit 1 ;}

for target in ${TEST_GITHUB_REPO_OWNER_GITHUBAPP} ${TEST_GITHUB_REPO_OWNER_WEBHOOK:-""};do
    [[ -z ${target} ]] && continue
    export GH_REPO=${target}
    export GH_HOST=$(echo ${TEST_GITHUB_API_URL}|sed 's,https://,,')
    export GH_ENTERPRISE_TOKEN=$(echo ${TEST_GITHUB_TOKEN})

    echo "Closing lingering PR on ${target}"
    for prn in $(gh pr list --jq .[].number --json number);do
        gh pr close ${prn}
    done
done

echo "Cleaning Namespaces"
{ kubectl get ns -o name|grep pac-e2e|sed 's/namespace.//'|xargs -r -P5 kubectl delete ns ;}

