---
title: Pipelines as Code Release Process
---
# Release process for Pipelines as Code

* Clear out the PR needed to be merged.
* Wait that CI is connected.
* Make sure the CI PAC cluster is up.
* Make sure you have gpg signing [setup](https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification) for your commits.
* You need to install python package semver :

```
pip3 install --user semver
```

* Use this script which should do most things : 

```
./hack/make-release.sh
```

* Choose a version if it's a major release/minor or patch release and let it push the new tags which should kick off pipelines as code [release pipelines](.tekton/release-pipeline.yaml).

* When it started you can follow it on the pac cluster : 

`tkn pr logs -n pipelines-as-code-ci -Lf`

* After a while (gorelease takes somet ime) If everything is fine you should
  have the new version set as pre-release in
  github.com/openshift-pipelines/pipelines-as-code/releases

* Edit the release like the other releases has been done with a snippet of the highlight of the release.

* Announce it on Slack (upstream/downstream)  and twitter.

## Packages

* [Arch AUR](https://aur.archlinux.org/packages/tkn-pac): Ping chmouel for an update

# Issues you may see 

* Sometime there may be some issues with system or others. If you need to rekick the release process you need to :

```shell
   git push --delete git@github.com:openshift-pipelines/pipelines-as-code release-1.2.3
   git push --delete git@github.com:openshift-pipelines/pipelines-as-code 1.2.3
   git tag --sign --force 1.2.3 
   git push git@github.com:openshift-pipelines/pipelines-as-code 1.2.3
```

* Some issues may be with the github token which may be expired or badly generated with a \n.
* Some other issues if you didn't do a git fetch -a origin before tagging so
  you don't have the latest commits from origin/main
