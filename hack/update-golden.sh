#!/usr/bin/env bash
# This script runs `go test -test.update-golden=true` for all packages that import `gotest.tools/v3/golden`.
# It updates golden files to match the current test output.
# Use this only when you intend to update the expected output.
# Note: This script targets unit tests only. To update golden files for e2e tests,
# run the e2e tests directly with the appropriate variables environemnt set and the -test.update-golden=true flag.
go test $(go list -f '{{ .ImportPath }} {{ .TestImports }}' ./pkg/... | grep gotest.tools/v3/golden | awk '{print $1}' | tr '\n' ' ') -test.update-golden=true
