#!/usr/bin/env bash
set -eux

cd $(dirname $0)
cd $(dirname $(pwd -P))

echo go run cmd/tknresolve/main.go  -f .tekton/push.yaml --generateName=true -p revision=main -p repo_url=https://github.com/openshift-pipelines/pipelines-as-code | kubectl create -f- -n pipelines-as-code-ci
