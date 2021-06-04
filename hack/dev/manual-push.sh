#!/usr/bin/env bash
set -eux

cd $(dirname $0)
cd $( dirname $(dirname $(pwd -P)))

go run cmd/tknresolve/main.go  -f .tekton/push.yaml --generateName=true -p revision=main -p repo_url=https://github.com/openshift-pipelines/pipelines-as-code
