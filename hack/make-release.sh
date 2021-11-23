#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

VERSION=${1:-""}
remote=git@github.com:openshift-pipelines/pipelines-as-code
CURRENTVERSION=$(git describe --tags $(git rev-list --tags --max-count=1))

bumpversion(){

   python3 -c "import semver" 2>/dev/null || {
       echo "install semver python module to bump version automatically: ie"
       echo "pip install --user semver"
       exit 1
   }

   read -p "Would you like to bump [M]ajor, Mi[n]or or [P]atch: " ANSWER
   if [[ ${ANSWER,,} == "m" ]];then
       mode=major
   elif [[ ${ANSWER,,} == "n" ]];then
       mode=minor
   elif [[ ${ANSWER,,} == "p" ]];then
       mode=patch
   else
       print "no or bad reply??"
       exit
   fi
   VERSION=$(python3 -c "import semver,sys;print(str(semver.VersionInfo.parse(sys.argv[1]).bump_${mode}()))"
             ${CURRENTVERSION})
   [[ -z ${VERSION} ]] && {
       echo "could not bump version automatically"
       exit
   }
}

echo "Current version is ${CURRENTVERSION}"

[[ -z ${VERSION} ]] && bumpversion
echo "Releasing ${VERSION}"

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
