---
title: Developer Resources
---
# How to get started developing for the Pipelines as Code project

## Please read the Code of conduct

It's important: <https://github.com/openshift-pipelines/pipelines-as-code/blob/main/code-of-conduct.md>

## Use the all in one install on kind to develop

It uses kind under docker. You start it with:

```shell
make allinone
```

When it finished you will have the following installed in your kind cluster:

- Kind Cluster deployment
- Internal registry to push to from `ko`
- A ingress controller with nginx for routing.
- Tekton and Dashboard installed with a ingress route.
- Pipelines as code deployed from your repo with ko.

By default it will try to install from
$GOPATH/src/github.com/openshift-pipelines/pipelines-as-code, if you want to
override it you can set the `PAC_DIRS` environment variable.

- It will deploy under the nip.io domain reflector, the URL will be :
  - <http://controller.paac-127-0-0-1.nip.io>
  - <http://dashboard.paac-127-0-0-1.nip.io>

- You will need to create secret yourself, if you have the [pass cli](https://www.passwordstore.org/)
  installed you can point to a folder which contains : github-application-id github-private-key webhook.secret
  As configured from your GitHub application. Configre `PAC_PASS_SECRET_FOLDER`
  environment variable to point to it.
  For example :

  ```shell
  pass insert github-app/github-application-id
  pass insert github-app/webhook.secret
  pass insert -m github-app/github-private-key
  ```

- If you need to redeploy your pac install (and only pac) you can do :

  ```shell
  ./install.sh -p
  ```

  or directly with ko :

  ```shell
  env KO_DOCKER_REPO=localhost:5000 ko apply -f ${1:-"config"} -B
  ```

- more flags: `-b` to only do the kind creation+nginx+docker image, `-r` to
  install from latest stable release (override with env variable `PAC_RELEASE`)
  instead of ko. `-c` will only do the pac configuration (ie: creation of
  secrets/ingress etc..)

## Gitea

Gitea is "unofficially" supported. You just need to configure Gitea the same way
you do for other webhook methods with a token.

Here is an example of a Gitea NS/CRD/Secret (set to empty):

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: gitea

---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: gitea
  namespace: gitea
spec:
  url: "https://gitea.my.com/owner/repo"
  git_provider:
    user: "git"
    url: "Your gitea installation URL, i.e: https://gitea.my.com/"
    secret:
      name: "secret"
      key: token
    webhook_secret:
      name: "secret"
      key: "webhook"
---
apiVersion: v1
kind: Secret
metadata:
  name: gitea-home-chmouel
  namespace: gitea
type: Opaque
stringData:
  token: "your token has generated in gitea"
  webhook: "" # make sure it's empty when you set this up on the interface and here
```

There is some gotchas with the webhook validation secret, Pipelines as Code
detect a Gitea install and let the user set a empty webhook secret (by default
it's enforced).

The `install.sh` script will by default spin up a new instance of GITEA to play
with and run the Gitea E2E tests.

You will need to create a Smee URL generated from <https://smee.io/new>
into the environment variable `TEST_GITEA_SMEEURL`.

The defaults are :

- URL: <https://localhost:3000/>
- Admin Username: pac
- Admin Password: pac

The E2E tests will automatically create repo using the admin username for each tests.

## Debugging controller

Create a [smee](https://smee.io) URL and point your app/webhook to it. Use
[gosmee](https://github.com/chmouel/gosmee) to forward the requests from github
to your locally installed controller (this can be either run on your debugger or
inside kind).

An option of gosmee is to save the replay to a directory with `--saveDir
/tmp/save`. If go to that directory a shell script will be created to replay
the request that was sent directly to the controller without having to go through
another push.

Use [snazy](https://github.com/chmouel/snazy) to watch the logs, it support pac
by adding some context like which github provider.

![snazy screenshot](/images/pac-snazy.png)

## Using the Makefile targets

Several target in the Makefile is available, if you need to run them
manually. You can list all the makefile targets with:

```shell
make help
```

For example to test and lint the go files :

```shell
make test lint-go
```

If you add a CLI command with help you will need to regenerate the golden files :

```shell
make update-golden
```

## Configuring the Pre Push Git checks

We are using several tools to verify that pipelines-as-code is up to a good
coding and documentation standard. We use pre-commit tools to ensure before you
send your PR that the commit is valid.

First you need to install pre-commit:

<https://pre-commit.com/>

It should be available as package on Fedora and Brew or install it with `pip`.

When you have it installed add the hook to your repo by doing :

```shell
pre-commit install
```

This will run several `hooks` on the files that has been changed before you
*push* to your remote branch. If you need to skip the verification (for whatever
reason), you can do :

```shell
git push --no-verify
```

or you can disable individual hook with the `SKIP` variable:

```shell
SKIP=lint-md git push
```

If you want to manually run on everything:

```shell
make pre-commit
```

## Developing the Documentation

Documentation is important to us, most of the time new features or change of
behaviour needs to include documentation part of the Pull Request.

We use hugo, if you want to preview your change, you need to install
[hugo](https://gohugo.io) and do a :

```shell
make docs-dev
```

this will start a hugo server with live preview of the docs on :

<https://localhost:1313>

When we push the release, the docs get rebuilded by CloudFare.

By default the website <https://pipelinesascode.com> only contains the "stable"
documentation. If you want to preview the dev documentation as from `main` you
need to go to this URL:

<https://main.pipelines-as-code.pages.dev>

## Documentation when we are doing the Release Process

- See here [release-process](release-process)

## Tools that are useful

Several tools are used on CI and in `pre-commit`, the non exhaustive list you
need to have on your system:

- [golangci-lint](https://github.com/golangci/golangci-lint) - For golang lint
- [yamllint](https://github.com/adrienverge/yamllint) - For YAML lint
- [pylint](https://readthedocs.org/projects/pylint/) - Python linter
- [black](https://github.com/psf/black) - Python code formatter check
- [vale](https://github.com/errata-ai/vale) - For grammar check
- [markdownlint](https://github.com/markdownlint/markdownlint) - For markdown lint
- [hugo](https://gohugo.io) - For documentation
- [ko](https://github.com/google/ko) - To rebuild and push change to kube cluster.
- [kind](https://kind.sigs.k8s.io/) - For local devs
- [snazy](https://github.com/chmouel/snazy) - To parse json logs nicely
- [pre-commit](https://pre-commit.com/) - For checking commits before sending it
  to the outer loop.
- [pass](https://www.passwordstore.org/) - For getting/storing secrets
- [gosmee](https://github.com/chmouel/gosmee) - For replaying webhooks

# Links

- [Jira Backlog](https://issues.redhat.com/browse/SRVKP-2144?jql=component%20%3D%20%22Pipeline%20as%20Code%22%20%20AND%20status%20!%3D%20Done)
- [Bitbucket Server Rest API](https://docs.atlassian.com/bitbucket-server/rest/7.17.0/bitbucket-rest.html)
- [GitHub API](https://docs.github.com/en/rest/reference)
- [Gitlab API](https://docs.gitlab.com/ee/api/api_resources.html)
