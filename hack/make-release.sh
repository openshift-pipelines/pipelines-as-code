#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

VERSION=${1:-""}
remote=git@github.com:openshift-pipelines/pipelines-as-code

[[ -z ${VERSION} ]] && {
    echo "need a version"
    exit 1
}

git status -uno|grep -q "nothing to commit" || {
    echo "there is change locally, commit them first"
    git status -uno
    exit 1
}

sed -i "s,\(https://github.com/openshift-pipelines/pipelines-as-code/blob/\)[^/]*,\1${VERSION}," README.md
sed -i "s/^VERSION=.*/VERSION=${VERSION}/" docs/install.md

git switch main

git commit -S -m "Update documentation for Release ${VERSION}" docs/install.md README.md || true
git tag -s ${VERSION} -m "Release ${VERSION}"
git push ${remote} refs/heads/main
git push --tags ${remote} refs/tags/${VERSION}
