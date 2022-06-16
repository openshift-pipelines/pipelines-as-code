---
title: CLI tkn-pac
weight: 10
---
# Pipelines as Code CLI

Pipelines as Code provide a powerful CLI designed to work with tkn plug-in.  `tkn pac` allows you to :

* `bootstrap`: quickly bootstrap a Pipelines as Code installation.
* `create`: create a new Pipelines as Code Repository definition.
* `delete`: delete an existing Pipelines as Code Repository definition.
* `generate`: generate a simple pipelinerun to get you started with Pipelines as Code.
* `list`: list Pipelines as Code Repositories.
* `describe`: describe a Pipelines as Code Repository and the runs associated with it.
* `resolve`: Resolve a pipelinerun as if it were executed by pipelines as code on service.
* `setup`: Setup a Git provider app or webhook with pipelines as code service.

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

It will start checking if you have installed Pipelines as Code and if not it
will ask you if you want to install (with `kubectl`) the latest stable
release. If you add the flag `--nightly` it will install the latest code ci
release.

Bootstrap detect the OpenShift Route automatically associated to the Pipelines
as code controller service.
If you don't have an OpenShift install it will ask you for your public URL (ie:
an ingress spec URL)
You can override the URL with the flag `--route-url`.
{{< /details >}}

{{< details "tkn pac bootstrap github-app" >}}

### bootstrap github-app

If you only want to create a GitHub application to use with Pipelines as Code
and not the full `bootstrap` excercise, you can use `tkn pac bootstrap
github-app` directly which will skip the installation and only create the
GitHub application and the secret with all the information needed in the
`pipelines-as-code` namespace.

{{< /details >}}

{{< details "tkn pac create repo" >}}

### Repository Creation

`tkn pac create repo` -- will create a new Pipelines as Code Repository
definition, a namespace where the pipelineruns command and configure webhook. It
will also generate a sample file with a [PipelineRun](/docs/guide/authoringprs)
in the `.tekton` directory called `pipelinerun.yaml` targeting the `main` branch
and the `pull_request` and `push` events. You can customize this by editing the
[PipelineRun](/docs/guide/authoringprs) to target a different branch or event.

If you haven't configured a provider previously, it will follow up with
questions if you want to configure a webhook for your provider of choice.
{{< /details >}}

{{< details "tkn pac delete repo" >}}

### Repository Deletion

`tkn pac delete repo` -- will delete a Pipelines as Code Repository definition.

You can specify the flag `--cascade` to optionally delete the attached secrets
(ie: webhook or provider secret) to the Pipelines as Code Repository definition.

{{< /details >}}

{{< details "tkn pac list" >}}

### Repository Listing

`tkn pac list` -- will list all the Pipelines as Code Repositories
definition and display the last status of the runs associated with it.
{{< /details >}}

{{< details "tkn pac describe" >}}

### Repository Describe

`tkn pac describe` -- will describe a Pipelines as Code Repository
definition and the runs associated with it.

On modern terminal (ie: [iTerm2](https://iterm2.com/), [Windows
Terminal](https://github.com/microsoft/terminal), GNOME-terminal and so on...)
the links are clickable with control+click or ⌘+click and will open the browser
to the UI URL to see the Pipelinerun associated with it.
{{< /details >}}

{{< details "tkn pac generate" >}}

### Generate

`tkn pac generate`: will generate a simple pipelinerun to get you started with
Pipelines as Code. It will try to be as smart as possible by detecting the
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
tkn pac resolve -f .tekton/pull-request.yaml|kubectl apply -f -
```

Combined with a kubernetes install running on your local machine (like[Code
Ready
Containers](https://developers.redhat.com/products/codeready-containers/overview)
or [Kubernetes Kind](https://kind.sigs.k8s.io/docs/user/quick-start/) ) you can
see your run in action without having to generate a new commit.

If you run the command from your source code repository it will try to detect
the current Git information and resolve the parameters like current revision or
branch. You can override those params with the `-p` option. For example if you
want to use a Git branch as revision and another repo name than the current repo
name you can just use :

`tkn pac resolve -f .tekton/pr.yaml -p revision=main -p repo_name=othername`

`-f` can as well accept a directory path rather than just a filename and grab
every `yaml`/`yml` from the directory.

You can specify multiple `-f` on the command line.

You need to verify that `git-clone` task (if you use it) can access the
repository to the SHA. Which mean if you test your current source code you need
to push it first before using `tkn pac resolve|kubectl apply`.

Compared with running directly on CI, you need to explicitly specify the list of
filenames or directory where you have the templates.
{{< /details >}}

{{< details "tkn pac setup github-webhook" >}}

### Setup GitHub Webhook

`tkn-pac setup github-webhook`: will let you set up a GitHub webhook to interact with Pipelines as Code

It will let you provide an option to create a Repository and configure it with the required secrets.
{{< /details >}}

{{< details "tkn pac setup gitlab-webhook" >}}

### Setup GitLab Webhook

`tkn-pac setup gitlab-webhook`: will let you set up a GitLab webhook to interact with Pipelines as Code.

It will let you provide an option to create a Repository and configure it with the required secrets.
{{< /details >}}

## Screenshot

![tkn-plug-in](/images/tkn-pac-cli.png)
