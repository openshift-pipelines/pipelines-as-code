---
title: Getting quickly started
weight: 2
---
# Getting started with Pipelines-as-Code

This guide will walk you through the process of getting started with Pipelines-as-Code.

This will start with the installation of Pipelines-as-Code on your cluster, then the creation of a GitHub Application, the creation of a Repository CRD to specify which repository you want to use with Pipelines-as-Code and finally we are going to create a simple Pull Request to test that configuration and see how the Pipelines-as-Code flow looks like.

## Prerequisites

* A Kubernetes cluster like [Kind](https://kind.sigs.k8s.io/), [Minikube](https://minikube.sigs.k8s.io/docs/start/), or [OpenShift Local](<https://developers.redhat.com/products/openshift-local/overview>)
* The OpenShift Pipelines Operator installed or [Tekton Pipelines](https://tekton.dev/docs/pipelines/install/) installed on your cluster
* The [Tekton CLI](https://tekton.dev/docs/cli/#installation) installed, and the Pipelines-as-Code CLI plugin ([tkn-pac](https://pipelinesascode.com/docs/guide/cli/#install)) installed

## Installing Pipelines-as-Code

There are multiple ways to install Pipelines-as-Code. The easiest way is to have it managed by the OpenShift Pipelines Operator, which installs Pipelines-as-Code by default.

You can also install it manually by applying the YAML files to your cluster or use `tkn pac bootstrap` to do it for you.

The `tkn pac bootstrap` command allows you to get started quickly and try out Pipelines-as-Code. If you are running this in production, you should consider installing Pipelines-as-Code using a GitOps tool like [OpenShift GitOps](https://www.openshift.com/learn/topics/gitops/) or [ArgoCD](https://argoproj.github.io/argo-cd/) following the manual installation instructions.

Let's get started with `tkn pac bootstrap`:

### Pipelines-as-Code Installation

```console
% tkn pac bootstrap
=> Checking if Pipelines-as-Code is installed.
🕵️ Pipelines-as-Code doesn't seem to be installed in the pipelines-as-code namespace.
? Do you want me to install Pipelines-as-Code v0.19.2? (Y/n) 
```

As soon as you get started, it will try to detect if you have Pipelines-as-Code installed. If it isn't installed, you will be asked if you want to install it.

You can go ahead and press `Y` to install it.

### Webhook Forwarder

The second question asks if you want to install the tool called [gosmee](https://github.com/chmouel/gosmee). This will only be asked if you are not running on OpenShift. This is not required, but since we need a way to have GitHub reach the Pipelines-as-Code controller from the internet, it's the easiest way to do it.

```console
Pipelines-as-Code does not install an Ingress object to allow the controller to be accessed from the internet. 
We can install a webhook forwarder called gosmee (https://github.com/chmouel/gosmee) using a https://hook.pipelinesascode.com URL. 
This will let your git platform provider (e.g., GitHub) reach the controller without requiring public access.
? Do you want me to install the gosmee forwarder? (Y/n) 
💡 Your gosmee forward URL has been generated: https://hook.pipelinesascode.com/zZVuUUOkCzPD
```

### Tekton Dashboard

Another question is only asked if you are not running on OpenShift. It will detect if you have the [Tekton Dashboard](https://github.com/tekton/dashboard) installed and if you want to use it for the links to show the logs or description of the PipelineRun.

```console
👀 We have detected a tekton dashboard install on http://dashboard.paac-127-0-0-1.nip.io
? Do you want me to use it? Yes
```

## GitHub Application Creation

The next step is to create a GitHub Application. You don't necessarily need to use Pipelines-as-Code with a GitHub Application (you can use it with a simple webhook as well), but it's the method that will give you the best experience.

You first need to enter the name of the GitHub Application you want to create. This name must be unique, so try to choose a name that is not too obvious and won't conflict with existing applications.

```console
? Enter the name of your GitHub application: My PAAC Application
```

As soon as you press Enter, the CLI will try to launch your web browser with the URL <https://localhost:8080>, which will display a button to create the GitHub Application. When you click the button, you will be redirected to GitHub and see the following screen:

![GitHub Application Creation](/images/github-app-creation-screen.png)

If you are satisfied with the name, you can click the "Create GitHub App" button. Then, you can return to your terminal where `tkn pac bootstrap` was launched. You will see that the GitHub Application has been created and a secret token has been generated.

```console
🔑 Secret pipelines-as-code-secret has been created in the pipelines-as-code namespace
🚀 You can now add your newly created application to your repository by going to this URL:

https://github.com/apps/my-paac-application

💡 Don't forget to run "tkn pac create repo" to create a new Repository CRD on your cluster.
```

You can visit the provided URL, click on `App settings`, and inspect how the GitHub Application has been created. `tkn pac bootstrap` will have configured everything you need to get started with Pipelines-as-Code, but you can customize settings if necessary.

### Creating a GitHub Repository and Adding the GitHub Application to It

As mentioned at the end of the `tkn pac bootstrap` command, you need to create a Repository CRD to specify which repository you want to use with Pipelines-as-Code.

If you don't have a repository, you can easily create one using this template:

<https://github.com/openshift-pipelines/pac-demo/generate>

Choose your username (e.g., `chmouel` for me) and a repository name (e.g., `pac-demo`), then click the "Create repository from template" button.

{{< hint info >}}
Pipelines-as-Code also works well with private repositories, but let's keep it simple for now and create a public repository.
{{< /hint >}}

Your repository is now created on GitHub at <https://github.com/yourusername/pac-demo>.

Next, you need to add the GitHub Application you created to this repository. You can do that by going to the GitHub App URL provided by `tkn pac bootstrap`, for example:

<https://github.com/apps/my-paac-application>

Click on the "Install" button and choose the repository you just created under your username.

![GitHub Application Installation](/images/github-app-install-application-on-repo.png)

## Creating a Repository CRD

Let's go back to the terminal and Checkout the newly created repository:

```console
git clone https://github.com/$yourusername/pac-demo
cd pac-demo
```

You are now ready to start the command:

```console
tkn pac create repo
```

`tkn pac create repo` try to be smart and helpful, as soon you are launching it will have detected the git information of the current repository and will ask you if you want to use those values.

```console
? Enter the Git repository url (default: https://github.com/chmouel/pac-demo):
```

You probably want to press enter here to use the default value, and then it will ask you to which namespace you will want to have your CI running. Again you can choose the default value here:

```console
? Please enter the namespace where the pipeline should run (default: pac-demo-pipelines):
! Namespace pac-demo-pipelines is not found
? Would you like me to create the namespace pac-demo-pipelines? (Y/n)
```

When this is done the process will be over it will generate a `Repository` CRD in your cluster and create a directory called `.tekton` with a file named `pipelinerun.yaml` in it.

```console
ℹ Directory .tekton has been created.
✓ We have detected your repository using the programming language Go.
✓ A basic template has been created in /tmp/pac-demo/.tekton/pipelinerun.yaml, feel free to customize it.
ℹ You can test your pipeline by pushing generated template to your git repository
```

Note that the command detected that you are using Go and created a basic template for you and had customized for the Go language (ie: it will add the [golangci-lint](https://hub.tekton.dev/tekton/task/golangci-lint) task to your PipelineRun).

Feel free to open the file `.tekton/pipelinerun.yaml` and customize it to your liking. The file has plenty of comments to help you get started.

## Creating a Pull Request

Now that we have our Repository CR created in our namespace our `.tekton/pipelinerun.yaml` generated we are now able to test if Pipelines-as-Code works.

Let's first create a branch to create a Pull Request from that branch.

```console
git checkout -b tektonci
```

Let's commit the `.tekton/pipelinerun.yaml` file and push it to our repository:

```console
git add .
git commit -m "Adding Tekton CI`
git push origin tektonci
```

{{< hint info >}}
We assume you have already set-up your system to be able to push to GitHub. If that's not the case see the official GitHub documentation on how to do it: <https://docs.github.com/en/get-started/getting-started-with-git/setting-your-username-in-git>
{{< /hint >}}

When the branch is pushed you can start creating a new Pull Request by going to the URL <https://github.com/yourusername/pac-demo/pull/new/tektonci>

As soon as you create the Pull Request you will see that the Pipelines-as-Code has been triggered:

![GitHub Application Installation](/images/github-app-install-CI-triggered.png)

You can click on the "Details" link to see the details of the running of the PipelineRun. `Pipelines-as-Code` will let you know that you can follow the logs on your Dashboard like Tekton Dashboard or OpenShift Pipelines Console or even via the tkn CLI on your cluster.

When the PipelineRun is finished you will see an error on that Detail screen:

![GitHub Application Installation](/images/github-app-install-CI-failed.png)

That was on purpose, we have detected some error on the Go code and GolangCI lint that is not correct. See how the error displayed link to the line of the code that is wrong. Pipelines-as-Code analyze the log error of the PipelineRun and try to match it to the line of the code that is wrong so you can easily fix it.

![GitHub Application Installation](/images/github-app-matching-annotations.png)

### Fixing the error

Let's go ahead and go back to our terminal and fix that error. Edit the file `main.go` select everything and  replace it with this content

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

commit this file and push it to your branch:

```console
git commit -a -m "Fixing the error"
git push origin tektonci
```

If you go back to your web browser you will see that the PipelineRun has been triggered again and this time it has succeeded:

![GitHub Application Installation](/images/github-app-install-CI-succeeded.png)

## Conclusion

You have now successfully, setu-up and used Pipelines-as-Code to run your CI on your repository. You can now go ahead and customize your `.tekton/pipelinerun.yaml` file to your liking and add more tasks to it.
