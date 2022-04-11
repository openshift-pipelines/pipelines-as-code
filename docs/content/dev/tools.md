---
title: Developers tools
---
# Tools to user as Openshift Pipelines as Code developer

## Pre-commit

We are using several tools to verify that pipelines-as-code is up to a good
coding and documentation standard.

First you need to install pre-commit:

<https://pre-commit.com/>

It should be available as package on Fedora and Brew or install it with `pip`.

When you have it installed add the hook to your repo by doing :

    pre-commit install

This will run several `hooks` on the files that has been changed before you
*push* to your remote branch. If you need to skip the verification (for whatever
reason), you can do :

    git push --no-verify

or you can disable individual hook with the `SKIP` variable:

    SKIP=lint-md git push

If you want to manually run on everything:

    make pre-commit

Several target in the Makefile is available, if you need to run them
manually. You can list all the makefile targets with:

    make help

For example to test and lint the go files :

    make test lint-go

## Tools

several tools are used on CI and in `pre-commit`, the non exhaustive list you
need to have on your system:

* [golangci-lint](https://github.com/golangci/golangci-lint)
* [yamllint](https://github.com/adrienverge/yamllint)
* [vale](https://github.com/errata-ai/vale)
* [markdownlint](https://github.com/golangci/golangci-lint)
