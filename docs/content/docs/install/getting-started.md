---
title: Getting Started Guide
weight: 2
---

# Getting started with Pipelines-as-Code

This guide will walk you through the process of getting started with Pipelines-as-Code.

This will start with the installation of Pipelines-as-Code on your cluster, then
the creation of a GitHub Application, the creation of a Repository CR to specify
which repository you want to use with Pipelines-as-Code, and finally, we will
create a simple Pull Request to test that configuration and see how the
Pipelines-as-Code flow works.

## Prerequisites

- A Kubernetes cluster like [Kind](https://kind.sigs.k8s.io/), [Minikube](https://minikube.sigs.k8s.io/docs/start/), or [OpenShift Local](https://developers.redhat.com/products/openshift-local/overview)
- The OpenShift Pipelines Operator installed or [Tekton Pipelines](https://tekton.dev/docs/pipelines/install/) installed on your cluster
- The [Tekton CLI](https://tekton.dev/docs/cli/#installation) installed, and the Pipelines-as-Code CLI plugin ([tkn-pac](https://pipelinesascode.com/docs/guide/cli/#install)) installed

## Installing Pipelines-as-Code

There are multiple ways to install Pipelines-as-Code. The easiest way is to have
it managed by the OpenShift Pipelines Operator, which installs Pipelines-as-Code
by default.

You can also install it manually by applying the YAML files to your cluster or
use `tkn pac bootstrap` to do it for you.

The `tkn pac bootstrap` command helps you start using Pipelines-as-Code quickly.

If you are running Pipelines-as-Code in production, you should consider installing
using a GitOps tool like [OpenShift
GitOps](https://www.openshift.com/learn/topics/gitops/) or
[ArgoCD](https://argoproj.github.io/argo-cd/) following the manual installation
instructions.

This guide uses `tkn pac bootstrap` to get you started.

{{< hint info >}}
Note: This assumes using the public GitHub instance. If you are using GitHub
Enterprise, you will need to use the `--github-hostname` flag with the hostname of your GitHub Enterprise instance.
{{< /hint >}}

### Pipelines-as-Code Installation

```bash
% tkn pac bootstrap
=> Checking if Pipelines-as-Code is installed.
🕵️ Pipelines-as-Code doesn't seem to be installed in the pipelines-as-code namespace.
? Do you want me to install Pipelines-as-Code v0.19.2? (Y/n)
```

As soon as you get started, `tkn pac bootstrap` tries to detect whether you have
Pipelines-as-Code installed. If it is not installed, you will be asked whether you
want to install it.

You can go ahead and press `Y` to install it.

### Webhook Forwarder

The second question asks if you want to install the tool called
[gosmee](https://github.com/chmouel/gosmee). This will only be asked if you are
not running on OpenShift.

This is not required, but considering the need to have GitHub reach our
Pipelines-as-Code controller from the internet, using `gosmee` is the most
straightforward way to do it. Note that while gosmee lets you get started
quickly, it is not intended for production use.

```console
Pipelines-as-Code does not install an Ingress object to allow the controller to be accessed from the internet.
We can install a webhook forwarder called gosmee (https://github.com/chmouel/gosmee) using a https://hook.pipelinesascode.com URL.
This will let your git platform provider (e.g., GitHub) reach the controller without requiring public access.
? Do you want me to install the gosmee forwarder? (Y/n)
💡 Your gosmee forward URL has been generated: https://hook.pipelinesascode.com/zZVuUUOkCzPD
```

### Tekton Dashboard

If you have the [Tekton Dashboard](https://github.com/tektoncd/dashboard)
installed and want to use it for the links to show the logs or
description of the PipelineRun. If you are running on OpenShift it will
automatically detect the OpenShift console Route and use it.

```console
👀 We have detected a tekton dashboard install on http://dashboard.paac-127-0-0-1.nip.io
? Do you want me to use it? Yes
```

## GitHub Application

The next step is to create a GitHub Application. You don't necessarily need to
use Pipelines-as-Code with a GitHub Application (you can use it with a simple
webhook as well), but it's the method that will give you the best experience.

You first need to enter the name of the GitHub Application you want to
create. This name must be unique, so try to choose a name that is not too
obvious and won't conflict with existing applications.

```console
? Enter the name of your GitHub application: My PAAC Application
```

As soon as you press Enter, the CLI will try to launch your web browser with the
URL <https://localhost:8080>, which will display a button to create the GitHub
Application. When you click the button, you will be redirected to GitHub and see
the following screen:

![GitHub Application Creation](/images/github-app-creation-screen.png)

Unless you want to change the name, you can click the "Create GitHub App"
button. Then, you can return to your terminal where `tkn pac bootstrap` was
launched. You will see that the GitHub Application has been created and a secret
token has been generated.

```console
🔑 Secret pipelines-as-code-secret has been created in the pipelines-as-code namespace
🚀 You can now add your newly created application to your repository by going to this URL:

https://github.com/apps/my-paac-application

💡 Don't forget to run "tkn pac create repository" to create a new Repository CR on your cluster.
```

You can visit your newly created GitHub Application by clicking on the URL.
If you click on the "App settings" button on that webpage, you can inspect
how the GitHub Application has been created. The `tkn pac bootstrap` command
will have configured everything you need to get started with Pipelines-as-Code,
but you can customize advanced settings there if necessary.

{{< hint info >}}
A useful tab to look at is the "Advanced" tab, this is where you can see all the recent deliveries of the GitHub app toward the Pipelines-as-Code controller. This is useful to debug if you have any issues with the communication between GitHub and the endpoint URL, or to investigate which events are being sent.
{{< /hint >}}

### Creating a GitHub Repository

As mentioned at the end of the `tkn pac bootstrap` command, you need to create a Repository CR to specify which repository you want to use with Pipelines-as-Code.

If you don't have a repository, you can easily create one using this template:

<https://github.com/openshift-pipelines/pac-demo/generate>

Choose your username (e.g., `chmouel` for me) and a repository name (e.g., `pac-demo`), then click the "Create repository from template" button.

{{< hint info >}}
Pipelines-as-Code also works well with [private repositories]({{< relref "/docs/guide/privaterepo.md" >}}), but let's keep it simple for now and create a public repository.
{{< /hint >}}

Your repository is now created on GitHub at <https://github.com/yourusername/pac-demo>.

### Install the GitHub Application in our Repository

Next, you need to add the GitHub Application you just created on your GitHub
repository. You can do this by following these steps:

1. Go to the GitHub App URL provided by `tkn pac bootstrap`, for example:
   [https://github.com/apps/my-paac-application](https://github.com/apps/my-paac-application).
2. Click on the "Install" button.
3. Choose the repository you just created under your username.

Here is an example of what it looks like for me:

![GitHub Application Installation](/images/github-app-install-application-on-repo.png)

### Check-out the newly created GitHub repository

Let's go back to the terminal and check-out with `git` the newly created repository:

```bash
git clone https://github.com/$yourusername/pac-demo
cd pac-demo
```

We navigate inside our repository since `tkn pac` will use the information from `git`
to provide some helpful defaults for the next commands we will execute.

## Create a Repository CR

{{< hint info >}}
A Repository CR is how you configure Pipelines-as-Code. A CR or Custom
Resource is a Kubernetes object that is not part of the core Kubernetes API. It's
a way to extend Kubernetes with new objects. In this case, we are using a CR to
specify which Repository URL (among other [settings]({{< relref
"/docs/guide/repositorycrd.md" >}})) we want to use with Pipelines-as-Code and
the namespace location is where our PipelineRuns will be run.
{{< /hint >}}

You are now ready to create a Repository CR with the command:

```bash
tkn pac create repository
```

The command tries to be smart and helpful. It detects the git information of the current repository and allows you to keep or change the values.

```console
? Enter the Git repository url (default: https://github.com/chmouel/pac-demo):
```

You probably want to press enter here to use the default value, and then it will
ask you to which namespace you will want to have your CI running. Again you can
choose the default value here:

```console
? Please enter the namespace where the pipeline should run (default: pac-demo-pipelines):
! Namespace pac-demo-pipelines is not found
? Would you like me to create the namespace pac-demo-pipelines? (Y/n)
```

When this is done, the process will be complete. It will generate a `Repository` CR in
your cluster and create a directory called `.tekton` with a file named
`pipelinerun.yaml` in it.

```console
ℹ Directory .tekton has been created.
✓ We have detected your repository using the programming language Go.
✓ A basic template has been created in /tmp/pac-demo/.tekton/pipelinerun.yaml, feel free to customize it.
ℹ You can test your pipeline by pushing the generated template to your git repository
```

Note that the `tkn pac create repository` command detected that the repository
is using the Go programming language and created a basic template for you to be
customized tailored for the Go programming language (ie: it will add the
[golangci-lint](https://artifacthub.io/packages/tekton-task/tekton-catalog-tasks/golangci-lint) linter as a
task to your PipelineRun).

Feel free to open the file `.tekton/pipelinerun.yaml` and inspect what it
does. The file has plenty of comments to help you understand how it works.

## Creating a Pull Request

Now that we have our Repository CR created in our namespace and our
`.tekton/pipelinerun.yaml` generated, we are now able to test whether
Pipelines-as-Code works.

Let's first create a branch to create a Pull Request from that branch.

```bash
git checkout -b tektonci
```

Let's commit the `.tekton/pipelinerun.yaml` file and push it to our repository:

```bash
git add .
git commit -m "Adding Tekton CI"
git push origin tektonci
```

{{< hint info >}}
We assume you have already set up your system to be able to push to GitHub. If that's not the case, see the official GitHub documentation on how to do it: <https://docs.github.com/en/get-started/getting-started-with-git/setting-your-username-in-git>
{{< /hint >}}

When the branch is pushed you can start creating a new Pull Request by going to
the URL (make sure yourusername is replaced with your username)
<https://github.com/yourusername/pac-demo/pull/new/tektonci>

As soon as you create the Pull Request you will see that Pipelines-as-Code has
been triggered and run on your Pull Request:

![GitHub Application Installation](/images/github-app-install-CI-triggered.png)

You can click on the "Details" link to see the details of the running of the
PipelineRun. `Pipelines-as-Code` will let you know that you can follow the logs
on your Dashboard like [Tekton Dashboard](https://github.com/tektoncd/dashboard)
or the OpenShift Pipelines
[Console](https://docs.openshift.com/container-platform/latest/web_console/web-console.html)
or if you prefer you can use [tekton CLI](https://tekton.dev/docs/cli/) to
follow the PipelineRun execution on your cluster.

When the PipelineRun is finished you will see an error on that Detail screen:

![GitHub Application Installation](/images/github-app-install-CI-failed.png)

That was on purpose. We have detected some linting errors in the Go code and
golangci-lint flagged it as incorrect. See how the error displayed links to the line
of the code that is wrong. Pipelines-as-Code analyzes the log error of the
PipelineRun and tries to match it to the line of the code that is wrong so you can
easily fix it.

![GitHub Application Installation](/images/github-app-matching-annotations.png)

### Fixing the error and pushing again

Let's go ahead and go back to our terminal to fix that error.

Edit the file `main.go`, select everything, and replace it with this content:

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    fmt.Fprintf(os.Stdout, "Hello world")
}
```

Commit this file and push it to your branch:

```bash
git commit -a -m "Errare humanum est, ignoscere divinum."
git push origin tektonci
```

If you go back to your web browser, you will see that the PipelineRun has been
triggered again and this time has succeeded:

![GitHub Application Installation](/images/github-app-install-CI-succeeded.png)

## Conclusion

Congratulations! You have now successfully set up Pipelines-as-Code for running
Continuous Integration on your repository. Now, you can freely proceed to
[customize]({{< relref "/docs/guide/authoringprs.md" >}}) your `.tekton/pipelinerun.yaml` file as per your preferences and
include additional tasks as needed.

### Summary

In this document, we have:

- [Installed]({{< relref "/docs/install/installation" >}}) Pipelines-as-Code on our Kubernetes Cluster.
- Created a [GitHub Application]({{< relref "/docs/install/github_apps" >}}).
- Created a [Repository CR]({{< relref "/docs/guide/repositorycrd" >}}).
- Generated a Pipelines-as-Code [PipelineRun]({{< relref "/docs/guide/authoringprs" >}}).
- [Run the PipelineRun]({{< relref "/docs/guide/running" >}}) on the Pull Request.
