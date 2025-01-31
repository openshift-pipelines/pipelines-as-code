---
title: Pipelines-as-Code Release Process
---
# Release process for Pipelines-as-Code

* Clear out the PR needed to be merged.
* Wait that CI is connected.
* Verify PAC CI cluster is up.
* Verify that you have gpg signing [setup](https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification) for your commits.

* Prepare to tag the release with a version, you need to choose between a major release/minor or patch release.

* If for example you choose to do the release 1.2.3 you tag it locally :

```shell
git tag v1.2.3
```

* And pushing it directly to the repo (you need access) :

```shell
% NOTESTS=ci git push git@github.com:openshift-pipelines/pipelines-as-code refs/tags/1.2.3
```

* When it started you can follow it on the pac cluster :

`tkn pr logs -n pipelines-as-code-ci -Lf`

* After a while (gorelease takes sometime) If everything is fine you should
  have the new version set as pre-release in
  <https://github.com/openshift-pipelines/pipelines-as-code/releases>

* Edit the release like the other releases has been done with a snippet of the highlight of the release.

* Announce it on Slack (upstream/downstream)  and twitter.

## Packages

* [Arch AUR](https://aur.archlinux.org/packages/tkn-pac): Ping chmouel for an update

# Issues you may see

* Sometimes, there may be some issues with system or others. If you need to re-kick the release process you need to :

```shell
   git tag --sign --force v1.2.3
   git push --force git@github.com:openshift-pipelines/pipelines-as-code v1.2.3
```

* Some issues may be with the GitHub token which may be expired or badly generated with a \n.
* Some other issues if you didn't do a `git fetch -a origin` before tagging so,
  you don't have the latest commits from origin/main
