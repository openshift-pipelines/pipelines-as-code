#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

VERSION=${1:-""}
remote=git@github.com:openshift-pipelines/pipelines-as-code
CURRENTVERSION=$(git describe --tags $(git rev-list --tags --max-count=1))

bumpversion() {
    mode=""
    current=$(git describe --tags $(git rev-list --tags --max-count=1))
    echo "Current version is ${current}"

    major=$(python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_major()))" ${CURRENTVERSION})
    minor=$(python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_minor()))" ${CURRENTVERSION})
    patch=$(python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_patch()))" ${CURRENTVERSION})

    echo "If we bump we get, Major: ${major} Minor: ${minor} Patch: ${patch}"
    read -p "To which version you would like to bump [M]ajor, Mi[n]or, [P]atch or Manua[l]: " ANSWER
    if [[ ${ANSWER,,} == "m" ]];then
       mode="major"
       elif [[ ${ANSWER,,} == "n" ]];then
            mode="minor"
       elif [[ ${ANSWER,,} == "p" ]];then
            mode="patch"
       elif [[ ${ANSWER,,} == "l" ]];then
            read -p "Enter version: " -e VERSION
            return
       else
           print "no or bad reply??"
           exit
       fi
            VERSION=$(python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_${mode}()))" ${CURRENTVERSION})
            [[ -z ${VERSION} ]] && {
                echo "could not bump version automatically"
                exit
            }
            echo "Releasing ${VERSION}"
}

[[ -z ${VERSION} ]] && bumpversion

git status -uno|grep -q "nothing to commit" || {
    echo "there is change locally, commit them first"
    git status -uno
    exit 1
}

sed -i "s,\(https://github.com/openshift-pipelines/pipelines-as-code/blob/\)[^/]*,\1${VERSION}," README.md
sed -i "s/^VERSION=.*/VERSION=${VERSION}/" docs/install.md

git fetch -a origin
git checkout -B main origin/main

git commit -S -m "Update documentation for Release ${VERSION}" docs/install.md README.md || true
git tag -s ${VERSION} -m "Release ${VERSION}"
git push ${remote} refs/heads/main
git push --tags ${remote} refs/tags/${VERSION}
