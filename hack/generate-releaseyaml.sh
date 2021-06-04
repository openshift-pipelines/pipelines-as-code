#!/usr/bin/env sh
set -euf

export TARGET_REPO=${TARGET_REPO:-quay.io/openshift-pipeline/pipelines-as-code}
export TARGET_BRANCH=${TARGET_BRANCH:-main}

for file in $(find config -maxdepth 1 -name '*.yaml'|sort -n);do
    head -1 ${file} | grep -q -- "---" || echo "---"
    sed -r "s,(.*image:.*)ko://github.com/openshift-pipelines/pipelines-as-code/cmd/.*,\1${TARGET_REPO}:${TARGET_BRANCH}\"," ${file}
done
