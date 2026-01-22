---
title: Contributor Resources
weight: 15
---

# How to get started developing for the Pipelines-as-Code project

## Please read the Code of conduct

It's important: <https://github.com/openshift-pipelines/pipelines-as-code/blob/main/code-of-conduct.md>

## Local Development Setup with startpaac

For local development, we recommend using [startpaac](https://github.com/openshift-pipelines/startpaac).

startpaac provides an interactive, modular setup that includes:

- Kind Cluster deployment
- Internal registry for `ko`
- Nginx ingress controller
- Tekton and Dashboard
- Pipelines as Code deployment
- Forgejo for local E2E testing

### Quick Start

```shell
git clone https://github.com/openshift-pipelines/startpaac
cd startpaac
./startpaac -a
```

See the [startpaac README](https://github.com/openshift-pipelines/startpaac) for configuration options and environment variables.

### Redeploying PAC

If you need to redeploy just Pipelines as Code, you can use ko directly:

```shell
env KO_DOCKER_REPO=localhost:5000 ko apply -f config -B
```

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

There are some gotchas with the webhook validation secret. Pipelines-as-Code
detects a Gitea install and lets the user set an empty webhook secret (by default
it's enforced).

startpaac will by default spin up a new instance of Forgejo (a Gitea fork) to play
with and run the Gitea E2E tests.

You will need to create a Hook URL generated from <https://hook.pipelinesascode.com/new>
into the environment variable `TEST_GITEA_SMEEURL`.

The defaults are:

- URL: <https://localhost:3000/>
- Admin Username: pac
- Admin Password: pac

The E2E tests will automatically create a repo using the admin username for each test.

## Debugging E2E

As long as you have the secrets set up, you should be able to run the e2e tests properly.
Gitea is the easiest to run (since they are self-contained). For the rest,
you will need to set up some environment variables.

See the [e2e on kind
workflow](https://github.com/openshift-pipelines/pipelines-as-code/blob/8f990bf5f348f6529deaa3693257907b42287a35/.github/workflows/kind-e2e-tests.yaml#L90)
for all the variables set by the provider.

By default, the E2E tests clean up after themselves. If you want to keep the
PR/MR open and the namespace where the test has been created, you can set the
`TEST_NOCLEANUP` environment variable to `true`.

## Debugging controller

Create a [hook](https://hook.pipelinesascode.com) URL and point your app/webhook to it. Use
[gosmee](https://github.com/chmouel/gosmee) to forward the requests from GitHub
to your locally installed controller (this can be either run on your debugger or
inside kind).

An option of gosmee is to save the replay to a directory with `--saveDir
/tmp/save`. If you go to that directory, a shell script will be created to replay
the request that was sent directly to the controller without having to go through
another push.

Use [snazy](https://github.com/chmouel/snazy) to watch the logs. It supports pac
by adding some context like which GitHub provider.

![snazy screenshot](/images/pac-snazy.png)

## Using the Makefile targets

Several targets are available in the Makefile if you want to run them
manually. You can list all the makefile targets with:

```shell
make help
```

For example, to test and lint the go files:

```shell
make test lint-go
```

We use [golden](https://pkg.go.dev/gotest.tools/v3/golden) files in our tests, for instance, to compare the output of CLI commands or other detailed tests. Occasionally, you may need to regenerate the golden files if you modify the output of a command. For unit tests, you can use this Makefile target:

```shell
make update-golden
```

Head over to the
[./test/README.md](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/test/README.md)
for more information on how to update the golden files on the E2E tests.

## Update OpenAPI Schemas

The CRD schemas are automatically generated from the Go code (generally
`pkg/apis/pipelinesascode/v1alpha1/types.go`). After modifying
any type definitions, you'll need to regenerate these schemas to update the CRD
in `config/300-repositories.yaml`.

When modifying types, ensure the validation logic is appropriate, then run:

```shell
make update-schemas
```

There is a PAC CI check that will ensure that the CRD is up to date with the go
code.

## Configuring the Pre Push Git checks

We are using several tools to verify that pipelines-as-code is up to a good
coding and documentation standard. We use pre-commit tools to ensure before you
send your PR that the commit is valid.

First, you need to install pre-commit:

<https://pre-commit.com/>

It should be available as a package on Fedora and Brew or install it with `pip`.

Once installed, add the hook to your repo by doing:

```shell
pre-commit install
```

This will run several `hooks` on the files that have been changed before you
_push_ to your remote branch. If you need to skip the verification (for whatever
reason), you can do:

```shell
git push --no-verify
```

or you can disable an individual hook with the `SKIP` variable:

```shell
SKIP=lint-md git push
```

If you want to manually run on everything:

```shell
make pre-commit
```

## Developing the Documentation

Documentation is important to us. Most of the time, new features or changes in
behavior need to include documentation as part of the Pull Request.

We use [hugo](https://gohugo.io). If you want to preview the changes you made
locally while developing, you can run this command:

```shell
make dev-docs
```

This will download a version of Hugo that is the same as what we use on
Cloudflare Pages (where [pipelinesascode.com](https://pipelinesascode.com) is
generated) and start the Hugo server with a live preview of the docs on:

<https://localhost:1313>

When we push the release, the docs get rebuilt automatically by CloudFare pages.

By default, the website <https://pipelinesascode.com> only contains the "stable"
documentation. If you want to preview the dev documentation as from `main`, you
need to go to this URL:

<https://main.pipelines-as-code.pages.dev>

There is a drop-down at the bottom of the page to let you change the older
major version.

### Documentation shortcode

The hugo-book theme has several shortcodes that are used to do different things
for the documentation.

See the demo site of hugo-book on how to use them here <https://github.com/alex-shpak/hugo-book#shortcodes>

And the demo on how to use them here:

<https://hugo-book-demo.netlify.app/>

We have as well some custom ones, you can see them in this directory:

<https://github.com/openshift-pipelines/pipelines-as-code/tree/main/docs/layouts/shortcodes>

See below on how to use them, feel free to grep around the documentation to see how they are actually used.

#### tech_preview

```markdown
{ {< tech_preview "Feature Name" >}}
```

This shortcode creates a red warning blockquote indicating that a feature is in Technology Preview status. It takes one parameter - the name of the feature. The output shows a warning message that the specified feature is not supported for production use and is provided for early testing and feedback.

#### support_matrix

```markdown
  { {< support_matrix github_app="true" github_webhook="true|false" gitea="true|false" gitlab="true|false" bitbucket_cloud="true|false" bitbucket_datacenter="true|false" >}}
```

This shortcode generates a compatibility table showing which Git providers support a particular feature. Each parameter accepts "true" or "false" values, displaying checkmarks (✅) or cross marks (❌) accordingly. The table lists all major Git providers (GitHub App, GitHub Webhook, Gitea, GitLab, Bitbucket Cloud, and Bitbucket Data Center) with their support status for the feature.

## Documentation when we are doing the Release Process

- See here [release-process]({{< relref "/docs/dev/release-process.md" >}})

## How to update all dependencies in Pipelines-as-Code

### Go Modules

Unless that's not possible, we try to update all dependencies to the
latest version as long as it's compatible with the Pipeline version as shipped by
OpenShift Pipelines Operator (which should be conservative).

Every time you update the Go modules, check if you can remove the `replace`
clause which pins a dependency to a specific version/commit or match the replace
to the tektoncd/pipeline version.

- Update all go modules:

  ```shell
  go get -u ./...
  make vendor
  ```

- Go to <https://github.com/google/go-github> and note the latest go version, for example: v59
- Open a file that uses the go-github library (i.e., pkg/provider/github/detect.go) and check the old version, for example: v56

- Run this sed command:

  ```shell
  find -name '*.go'|xargs sed -i 's,github.com/google/go-github/v56,github.com/google/go-github/v59,'
  ```

- This will update everything. Sometimes the library ghinstallation is not
updated with the new version, so you will need to keep the old version kept in
there. For example, you will get this kind of error:

  ```text
  pkg/provider/github/parse_payload.go:56:33: cannot use &github.InstallationTokenOptions{…} (value of type *"github.com/google/go-github/v59/github".InstallationTokenOptions) as *"github.com/google/go-github/v57/github".InstallationTokenOptions value in assignment
  ```

- Check that everything compiles and tests are passing with this command:

  ```shell
  make allbinaries test lint
  ```

- Some structs need to be updated. Some of them are going to fail as
  deprecated, so you will need to figure out how to update them. Don't be lazy and avoid the
  update with a nolint or a pin to a dep. You only delay the inevitable until
  the problem comes back and hits you harder.

### Go version

- Check that the go version is updated to the latest RHEL version:

  ```shell
  docker pull golang
  docker run golang go version
  ```

- If this is not the same as what we have in go.mod, then you need to update the go.mod version. Then you need to update, for example, here 1.20:

  ```shell
  go mod tidy -go=1.20
  ```

- Grep for the image go-toolset everywhere with:

  ```shell
  git grep golang:
  ```

  and change the old version to the new version

### Update the pre-commit rules

  ```shell
  pre-commit autoupdate
  ```

### Update the vale rules

  ```shell
  vale sync
  make lint-md
  ```

## Tools that are useful

Several tools are used in CI and `pre-commit`. The non-exhaustive list you
need to have on your system:

- [golangci-lint](https://github.com/golangci/golangci-lint) - For golang lint
- [yamllint](https://github.com/adrienverge/yamllint) - For YAML lint
- [shellcheck](https://www.shellcheck.net/) - For shell scripts linting
- [ruff](https://github.com/astral-sh/ruff) - Python code formatter check
- [vale](https://github.com/errata-ai/vale) - For grammar check
- [markdownlint](https://github.com/markdownlint/markdownlint) - For markdown lint
- [codespell](https://github.com/codespell-project/codespell) - For code spelling
- [gitlint](https://github.com/jorisroovers/gitlint) - For git commit messages lint
- [hugo](https://gohugo.io) - For documentation
- [ko](https://github.com/google/ko) - To rebuild and push change to kube cluster.
- [kind](https://kind.sigs.k8s.io/) - For local devs
- [snazy](https://github.com/chmouel/snazy) - To parse json logs nicely
- [pre-commit](https://pre-commit.com/) - For checking commits before sending it
  to the outer loop.
- [pass](https://www.passwordstore.org/) - For getting/storing secrets
- [gosmee](https://github.com/chmouel/gosmee) - For replaying webhooks

## Target architecture

- We target arm64 and amd64. The dogfooding is on arm64, so we need to ensure
that all jobs and docker images used in the .tekton PipelineRuns are built
for arm64.
- A GitHub action is using [ko](https://ko.build/) to build the amd64 and arm64 images whenever there is
a push to a branch or for a release.

## LLM Assistance Disclosure

When submitting a pull request to Pipelines-as-Code, contributors must disclose
any AI/LLM assistance used during development. This promotes transparency and
proper attribution in our collaborative development environment.

### Python dependencies

```shell
cd ./hack/pr-ci
uv lock -U
```

### Required Disclosure

All contributors must:

1. **Check the appropriate boxes** in the PR template's "🤖 AI Assistance"
   section
2. **Specify which LLM was used** (GitHub Copilot, ChatGPT, Claude, Cursor,
   Gemini, etc.)
3. **Indicate the extent of assistance** (documentation, code generation, PR
   description, etc.)
4. **Add Co-authored-by trailers** to commit messages when AI significantly
   contributed to the code

### Adding Co-authored-by Trailers

For commits where AI contributed significantly to the code, add appropriate
`Co-authored-by` trailers to your commit messages. You can use our helper
script to automate this process:

```shell
./hack/add-llm-coauthor.sh
```

This interactive script will:

- Help you select commits that used AI assistance
- Choose which AI assistants to credit
- Automatically add proper `Co-authored-by` trailers to your commit messages

**Examples of Co-authored-by trailers:**

```text
Co-authored-by: Claude <noreply@anthropic.com>
Co-authored-by: ChatGPT <noreply@chatgpt.com>
Co-authored-by: Cursor <cursor@users.noreply.github.com>
Co-authored-by: Copilot <Copilot@users.noreply.github.com>
Co-authored-by: Gemini <gemini@google.com>
```

### Why We Require This

- **Transparency**: Helps reviewers understand the development process
- **Attribution**: Properly credits AI tools that significantly contributed
- **Learning**: Helps the team understand effective AI-assisted development patterns
- **Compliance**: Meets organizational requirements for AI tool usage tracking

See the [PR template](.github/pull_request_template.md) for complete details on
the AI assistance disclosure requirements.
