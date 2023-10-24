#!/usr/bin/env python3
# Take a version as first argument (current)
# Take a set of version delimited by commas as second argument
# If the current is higher than all other versions return true otherwise false
# example:
# % python3 ./hack/compare-versions.py v1.1.0 v0.1.1,v0.2.2,v1.0.1
# true
# % python3 ./hack/compare-versions.py 0.1.0 0.1.1,0.2.2,1.0.1
# false
# semantic numbers with v is supported
import sys

# pylint: disable=no-name-in-module
from packaging import version

if len(sys.argv[1]) == 0:
    print("Usage compare-versions.py <current> <all versions separated by comma>")
    sys.exit(1)

current = sys.argv[1]
all_versions = sys.argv[2].split(",")

for i in all_versions:
    if version.parse(current) < version.parse(i):
        print("false")
        sys.exit(1)

print("true")
