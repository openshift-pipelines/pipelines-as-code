---
title: CLI tkn-pac
weight: 100
---
# Pipelines-as-Code CLI

Pipelines-as-Code provide a powerful CLI designed to work as a plug-in to the [Tekton CLI (tkn)](https://github.com/tektoncd/cli).

`tkn pac` allows you to :

* `bootstrap`: quickly bootstrap a Pipelines-as-Code installation.
* `create`: create a new Pipelines-as-Code Repository definition.
* `delete`: delete an existing Pipelines-as-Code Repository definition.
* `generate`: generate a simple pipelinerun to get you started with Pipelines-as-Code.
* `list`: list Pipelines-as-Code Repositories.
* `logs`: show the logs of a PipelineRun form a Repository CRD.
* `describe`: describe a Pipelines-as-Code Repository and the runs associated with it.
* `resolve`: Resolve a pipelinerun as if it were executed by pipelines as code on service.
* `webhook`: Updates webhook secret.
* `info`: Show information (currently only about your installation with `info install`).

## Install

{{< tabs "installbinary" >}}
{{< tab "Binary" >}}
You can grab the latest binary directly for your operating system from the
[releases](https://github.com/openshift-pipelines/pipelines-as-code/releases)
page.

Available operating systems are :

* MacOS - M1 and x86 architecture
* Linux - 64bits - RPM, Debian packages and tarballs.
* Linux - ARM 64bits - RPM, Debian packages and tarballs.
* Windows - Arm 64 Bits and x86 architecture.

{{< hint info >}}
On windows tkn-pac will look for the kubernetes config in `%USERPROFILE%\.kube\config` on Linux and MacOS it will use the standard $HOME/.kube/config.
{{< /hint >}}

{{< /tab >}}

{{< tab "Homebrew" >}}
tkn pac plug-in is available from HomeBrew as a "Tap", you simply need to run this command to install it :

```shell
brew install openshift-pipelines/pipelines-as-code/tektoncd-pac
```

and if you need to upgrade it :

```shell
brew upgrade openshift-pipelines/pipelines-as-code/tektoncd-pac
```

`tkn pac` plug-in is compatible with [Homebrew on Linux](https://docs.brew.sh/Homebrew-on-Linux)

{{< /tab >}}
{{< tab "Container" >}}
`tkn-pac` is available as a docker container, :

```shell
# use docker
podman run -e KUBECONFIG=/tmp/kube/config -v ${HOME}/.kube:/tmp/kube \
     -it  ghcr.io/openshift-pipelines/tkn-pac:stable tkn-pac help
```

{{< /tab >}}

{{< tab "GO" >}}
If you want to install from the Git repository you can just do :

```shell
go install github.com/openshift-pipelines/pipelines-as-code/cmd/tkn-pac
```

{{< /tab >}}

{{< tab "Arch" >}}
You can install the `tkn pac` plugin from the [Arch User
Repository](https://aur.archlinux.org/packages/tkn-pac/) (AUR) with your
favourite AUR installer like `yay` :

```shell
yay -S tkn-pac
```

{{< /tab >}}

{{< /tabs >}}

## Commands

{{< details "tkn pac bootstrap" >}}

### bootstrap

`tkn pac bootstrap` command will help you to get started installing and
configuring Pipelines as code. It currently supports the following providers:

* GitHub Application on public GitHub
* GitHub Application on GitHub Enterprise

It will start checking if you have installed Pipelines-as-Code and if not it
will ask you if you want to install (with `kubectl`) the latest stable
release. If you add the flag `--nightly` it will install the latest code ci
release.

Bootstrap detect the OpenShift Route automatically associated to the Pipelines
as code controller service and uses this as endpoint for the created GitHub
application.

You can use the `--route-url` flag to replace the OpenShift Route URL or specify
a custom URL on an
[Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) in a
Kubernetes cluster.

The OpenShift console is automatically detected, on Kubernetes, `tkn-pac` will
attempt to detect the tekton-dashboard Ingress URL and let you choose to use it
as the endpoint for the created GitHub application.

If your cluster is not accessible to the internet, Pipelines-as-Code provides an
option to install a webhook forwarder called
[gosmee](https://github.com/chmouel/gosmee). This forwarder enables connectivity
between the Pipelines-as-Code controller and GitHub without requiring an
internet connection. In this scenario, it will set up a forwarding URL on
<https://hook.pipelinesascode.com> and set it up on GitHub. For OpenShift, it
will not prompt you unless you explicitly specify the `--force-gosmee` flag
(which can be useful if you are running [OpenShift Local](https://developers.redhat.com/products/openshift-local/overview) for instance).

{{< /details >}}

{{< details "tkn pac bootstrap github-app" >}}

### bootstrap github-app

If you only want to create a GitHub application to use with Pipelines-as-Code
and not the full `bootstrap` exercise, you can use `tkn pac bootstrap
github-app` directly which will skip the installation and only create the
GitHub application and the secret with all the information needed in the
`pipelines-as-code` namespace.

{{< /details >}}

{{< details "tkn pac create repo" >}}

### Repository Creation

`tkn pac create repo` -- Creates a new Pipelines-as-Code `Repository` custom resource definition,
With a Git repository to execute pipelineruns based on Git events. It
will also generate a sample file with a [PipelineRun](/docs/guide/authoringprs)
in the `.tekton` directory called `pipelinerun.yaml` targeting the `main` branch
and the `pull_request` and `push` events. You can customize this by editing the
[PipelineRun](/docs/guide/authoringprs) to target a different branch or event.

If you haven't configured a provider previously, it will follow up with
questions if you want to configure a webhook for your provider of choice.
{{< /details >}}

{{< details "tkn pac delete repo" >}}

### Repository Deletion

`tkn pac delete repo` -- will delete a Pipelines-as-Code Repository definition.

You can specify the flag `--cascade` to optionally delete the attached secrets
(ie: webhook or provider secret) to the Pipelines-as-Code Repository definition.

{{< /details >}}

{{< details "tkn pac list" >}}

### Repository Listing

`tkn pac list` -- will list all the Pipelines-as-Code Repositories
definition and display the last or the current status (if its running) of the
PipelineRun associated with it.

You can add the option `-A/--all-namespaces` to list all repositories across the
cluster. (you need to have the right for it).

You can select the repositories by labels with the `-l/--selectors` flag.

You can choose to display the real time as RFC3339 rather than the relative time
with the `--use-realtime` flag.

On modern terminal (ie: OSX Terminal, [iTerm2](https://iterm2.com/), [Windows
Terminal](https://github.com/microsoft/terminal), GNOME-terminal, kitty and so
on...) the links become clickable with control+click or ⌘+click (see the
documentation of your terminal for more details) and will open the browser
to the console/dashboard URL to see the details of the Pipelinerun associated
with it.

{{< /details >}}

{{< details "tkn pac describe" >}}

### Repository Describe

`tkn pac describe` -- will describe a Pipelines-as-Code Repository
definition and the runs associated with it.

You can choose to display the real time as RFC3339 rather than the relative time
with the `--use-realtime` flag.

When the last PipelineRun has failure it will print the last 10 lines of every
tasks associated with the PipelineRun thas has been failed highlightign the
`ERROR` or `FAILURE` and other patterns.

If you  want to show the failures of another PipelineRun rather than the last
one you can use the `--target-pipelinerun` or `-t` flag for that.

On modern terminal (ie: OSX Terminal, [iTerm2](https://iterm2.com/), [Windows
Terminal](https://github.com/microsoft/terminal), GNOME-terminal, kitty and so
on...) the links become clickable with control+click or ⌘+click (see the
documentation of your terminal for more details) and will open the browser
to the console/dashboard URL to see the details of the Pipelinerun associated
with it.

{{< /details >}}

{{< details "tkn pac logs" >}}

### Logs

`tkn pac logs` -- will show the logs attached to a Repository.

If you don't specify a repository on the command line it will ask you to choose
one or auto select it if there is only one.

If there is multiple PipelineRuns attached to the Repo it will ask you to choose
one or auto select it if there is only one.

If you add the `-w` flag it will open the console or the dashboard URL to the log.

The [`tkn`](https://github.com/tektoncd/cli) binary needs to be installed to show
the logs.
{{< /details >}}

{{< details "tkn pac generate" >}}

### Generate

`tkn pac generate`: will generate a simple pipelinerun to get you started with
Pipelines-as-Code. It will try to be as smart as possible by detecting the
current Git information if you run the command from your source code.

It has some basic language detection and add extra task depending on the
language. For example if it detects a file named `setup.py` at the repository
root it will add the [pylint task](https://hub.tekton.dev/tekton/task/pylint) to
the generated pipelinerun.
{{< /details >}}

{{< details "tkn pac resolve" >}}

### Resolve

`tkn-pac resolve`: will run a pipelinerun as if it were executed by pipelines
as code on service.

For example if you have a pipelinerun in the `.tekton/pull-request.yaml` file
you can run the command `tkn-pac resolve` to see it running:

```yaml
tkn pac resolve -f .tekton/pull-request.yaml -o /tmp/pull-request-resolved.yaml && kubectl create -f /tmp/pull-request-resolved.yaml
```

Combined with a kubernetes install running on your local machine (like[Code
Ready
Containers](https://developers.redhat.com/products/codeready-containers/overview)
or [Kubernetes Kind](https://kind.sigs.k8s.io/docs/user/quick-start/) ) you can
see your run in action without having to generate a new commit.

If you run the command from your source code repository it will try to detect
the parameters (like the revision or branch_name) using the information from the
Git repository.

You can override the parameters with the `-p` flag.

For example if you want to use a Git branch as revision and another repo name
than the current repo name you can just use :

`tkn pac resolve -f .tekton/pr.yaml -p revision=main -p repo_name=othername`

`-f` can as well accept a directory path rather than just a filename and grab
every `yaml` or `yml` files from that directory.

Multiple `-f` arguments are accepted to provide multiple files on the command line.

You need to verify that `git-clone` task (if you use it) can access the
repository to the SHA. Which mean if you test your current source code you need
to push it first before using `tkn pac resolve|kubectl create -`.

Compared with running directly on CI, you need to explicitly specify the list of
filenames or directory where you have the templates.

On certain clusters, the conversion from v1beta1 to v1 in Tekton may not
function correctly, leading to errors when applying the resolved PipelineRun on
a different cluster that doesn't have the bundle feature enabled. To resolve
this issue, you can use the `--v1beta1` flag (or `-B` for short) to explicitly
output the PipelineRun as v1beta1 and work around the error.

When you run the resolver it will try to detect if you have a `{{
git_auth_secret }}` string inside your template and if there is a match it will
ask you to provide a Git provider token.

If you already have an existing secret created in your namespace matching your
repository URL it will use it.

You can explicitly provide a token on the command line with the `-t` or
`--providerToken` flag, or you can set the environment variable
`PAC_PROVIDER_TOKEN` and it will use it instead of asking you.

With the `--no-secret` flag you can completely skip any secret generation.

There is no clean-up of the secret after the run.

{{< /details >}}

{{< details "tkn pac webhook add" >}}

### Configure and create webhook secret for GitHub, GitLab and Bitbucket Cloud provider

`tkn-pac webhook add [-n namespace]`: Allows you to add new webhook secret for a given provider and update the value of the new webhook secret in the existing `Secret` object used to interact with Pipelines-as-Code

{{< /details >}}

{{< details "tkn pac webhook update-token" >}}

### Update provider token for existing webhook

`tkn pac webhook update-token [-n namespace]`: Allows you to update provider token for an existing `Secret` object to interact with Pipelines-as-Code.

{{< /details >}}

{{< details "tkn pac info install" >}}

### Installation Info

The `tkn pac info` command provides information about your Pipelines-as-Code
installation, including its location and version.

By default, it displays the version of the Pipelines-as-Code controller and the
namespace where Pipelines-as-Code is installed. This information is accessible
to all users on the cluster through a special ConfigMap named
`pipelines-as-code-info`. This ConfigMap has broad read access in the namespace
where Pipelines-as-Code is installed.

If you are a cluster admin, you can also view an overview of all created
Repositories CR on the cluster, along with their associated URLs.

As an admin, if your installation is set up with a [GitHub
App](../../install/github_apps), you can see the details of the installed
application and other relevant information, such as the URL endpoint configured
for your GitHub App. By default, this will display information from the public
GitHub API, but you can specify a custom GitHub API URL using the
`--github-api-url` argument.

{{< /details >}}

{{< details "tkn pac info globbing" >}}

### Test globbing pattern

The `tkn pac info globbing` command allows you to test glob patterns to see if
they match, for example, when using the `on-patch-change` annotation.

Here how it works, this example:

```bash
tkn pac info globbing 'docs/***/*.md'
```

will match all markdown files in the docs directory and its subdirectories if
present in the current directory.

By default, it tests the glob pattern against the current directory unless you
specify the `-d` or `--dir` flag to test against a different directory.

The first argument is the glob pattern to test (you will be prompted for it if
you don't provide one) as specified by the [glob
library](https://github.com/gobwas/glob?tab=readme-ov-file#example).

If you want to test against a string to test other annotation that uses globbing
patterns (like `on-target-branch` annotation) you can use the `-s` or `--string`
flag.

For example this will test if the globbing expression `refs/heads/*` matches
`refs/heads/main`:

```bash
tkn pac info globbing -s "refs/heads/main" "refs/heads/*"
```

#### Example

```bash
tkn pac info globbing 'docs/***/*.md'
```

This will match all markdown files in the docs directory and its subdirectories if
present in the current directory.

You can specify a different directory than the current one by using the -d/--dir flag.

{{< /details >}}

## Screenshot

![tkn-plug-in](/images/tkn-pac-cli.png)
