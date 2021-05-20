#!/usr/bin/env sh
# Chmouel Boudjnah <chmouel@chmouel.com>
set -euf

export TARGET_REPO=${TARGET_REPO:-quay.io/openshift-pipeline/pipelines-as-code}
export TARGET_BRANCH=${TARGET_BRANCH:-main}

for file in $(find config -maxdepth 1 -name '*.yaml');do
    [[ ${file} != "---"* ]] &&  echo "---"
    sed -r "s,(.*image:.*)ko://github.com/openshift-pipelines/pipelines-as-code/cmd/.*,\1${TARGET_REPO}:${TARGET_BRANCH}\"," ${file}
done
