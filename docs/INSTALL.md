# Installation Guides

Pipelines-as-Code support different installation method to WebVCS platforms (i.e: GitHub, Bitbucket etc..)

The preferred method to use Pipelines-as-Code is with
[GitHub Application](https://docs.github.com/en/developers/apps/getting-started-with-apps/about-apps).

Refers to the end of Documentation for the other WebVCS installations

## Install Pipelines-as-Code as GitHub Application

In order to install and use Pipelines-as-Code as GitHub application, you need to

* Install the Pipelines-as-Code infrastructure on your cluster
* Create a Pipelines-as-Code GitHub App on your GitHub account or organization
* Configure Pipelines-as-Code on your cluster to access the GitHub App

Here is a video walkthrough on the install process :

[![Pipelines as Code Install Walkthought](https://img.youtube.com/vi/d81rIHNFjJM/0.jpg)](https://www.youtube.com/watch?v=d81rIHNFjJM)

### Install Pipelines as Code infrastructure

To install Pipelines as Code on your cluster you simply need to run this command :

```shell
VERSION=0.4.1
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-$VERSION/release-$VERSION.yaml
```

If you would like to install the current development version you can simply install it like this :

```shell
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/nightly/release.yaml
```

It will apply the release.yaml to your kubernetes cluster, creating the admin namespace `pipelines-as-code`, the roles
and all other bits needed.

The `pipelines-as-code` namespace is where the Pipelines-as-Code infrastructure runs and is supposed to be accessible
only by the admin.

The Route for the EventListener URL is automatically created when you apply the release.yaml. You will need to grab the
url for the next section when creating the GitHub App. You can run this command to get the route created on your
cluster:

```shell
echo https://$(oc get route -n pipelines-as-code el-pipelines-as-code-interceptor -o jsonpath='{.spec.host}')
```

### Create a Pipelines-as-Code GitHub App

You should now create a Pipelines-as-Code GitHub App which acts as the integration point with OpenShift Pipelines and
brings the Git workflow into Tekton pipelines. You need the webhook of the GitHub App pointing to your Pipelines-as-Code
EventListener route endpoint which would then trigger pipelines on GitHub events.

* Go to https://github.com/settings/apps (or *Settings > Developer settings > GitHub Apps*) and click on **New GitHub
  App** button
* Provide the following info in the GitHub App form
    * **GitHub Application Name**: `OpenShift Pipelines`
    * **Homepage URL**: *[OpenShift Console URL]*
    * **Webhook URL**: *[the EventListener route URL copies in the previous section]*
    * **Webhook secret**: *[an arbitrary secret, you can generate one with `openssl rand -hex 20`]*

* Select the following repository permissions:
    * **Checks**: `Read & Write`
    * **Contents**: `Read & Write`
    * **Issues**: `Read & Write`
    * **Metadata**: `Readonly`
    * **Pull request**: `Read & Write`

* Select the following organization permissions:
    * **Members**: `Readonly`
    * **Plan**: `Readonly`

* Select the following user permissions:
    * Commit comment
    * Issue comment
    * Pull request
    * Pull request review
    * Pull request review comment
    * Push

> You can see a screenshot of how the GitHub App permissions look like [here](https://user-images.githubusercontent.com/98980/124132813-7e53f580-da81-11eb-9eb4-e4f1487cf7a0.png)

* Click on **Create GitHub App**.

* Take note of the **App ID** at the top of the page on the details page of the GitHub App you just created.

* In **Private keys** section, click on **Generate Private key* to generate a private key for the GitHub app. It will
  download automatically. Store the private key in a safe place as you need it in the next section and in future when
  reconfiguring this app to use a different cluster.

### Configure Pipelines-as-Code on your cluster to access the GitHub App

In order for Pipelines-as-Code to be able to authenticate to the GitHub App and have the GitHub App securely trigger the
Pipelines-as-Code webhook, you need to create a Kubernetes secret containing the private key of the GitHub App and the
webhook secret of the Pipelines-as-Code as it was provided when you created the GitHub App in the previous section. This
secret
is [used to generate](https://docs.github.com/en/developers/apps/building-github-apps/identifying-and-authorizing-users-for-github-apps)
a token on behalf of the user running the event and make sure to validate the webhook via the webhook secret.

Run the following command and replace:

* `APP_ID` with the GitHub App **App ID** copied in the previous section
* `WEBHOOK_SECRET` with the webhook secret provided when created the GitHub App in the previous section
* `PATH_PRIVATE_KEY` with the path to the private key that was downloaded in the previous section

```bash
kubectl -n pipelines-as-code create secret generic pipelines-as-code-secret \
        --from-literal github-private-key="$(cat PATH_PRIVATE_KEY)" \
        --from-literal github-application-id="APP_ID" \
        --from-literal webhook.secret="WEBHOOK_SECRET"
```

### GitHub Enterprise

Pipelines as Code supports Github Enterprise.

You don't need to do anything special to get Pipelines as code working with GHE. Pipelines as code will automatically
detects the header as set from GHE and use it the GHE API auth url instead of the public github.

## Install Pipelines-as-Code as a GitHub Webhook

If you are not able to create a GitHub application you can install Pipelines-as-Code on your repository as a
[GitHub Webhook](https://docs.github.com/en/developers/webhooks-and-events/webhooks/creating-webhooks).

Using Pipelines as Code via Github webhook does not give you access to the GitHub CheckRun API, therefore the status of
the tasks will be added as a Comment of the PR and not via the **Checks** Tab.

* You have to first install the Pipelines-as-Code infrastructure as detailled
  here : [Install infrastructure](INSTALL.md#install-pipelines-as-code-infrastructure)

* You will have to generate a personal token for Pipelines-as-Code Github API operations. Follow this guide to create a
  personal token :

<https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token>

The only permission needed is the *repo* permission. Make sure you note somewhere the generated token or otherwise you
will have to recreate it.

* Go to you repository or organisation setting and click on *Hooks* and *"Add webhook"* links.

* Set the payload URL to the event listenner public URL. On OpenShift you can get the public URL of the
  Pipelines-as-Code eventlistenner like this :

  ```shell
  echo https://$(oc get route -n pipelines-as-code el-pipelines-as-code-interceptor -o jsonpath='{.spec.host}')
  ```

* Add a secret or generate a random one with this command  :

  ```shell
  openssl rand -hex 20
  ```

* [Refer to this screenshot](./images/pac-direct-webhook-create.png) on how to configure the Webhook. The individual
  events to select are :
    * Commit comments
    * Issue comments
    * Pull request reviews
    * Pull request
    * Pushes

* On your cluster you need create the webhook secret as generated previously in the *pipelines-as-code* namespace.

```shell
kubectl -n pipelines-as-code create secret generic pipelines-as-code-secret \
        --from-literal webhook.secret="$WEBHOOK_SECRET_AS_GENERATED"
```

* You are now able to create a Repository CRD. The repository CRD will have a Secret that contains the Personal token as
  generated and Pipelines as Code will know how to use it for GitHub API operations.

    - First create the secret with the personal token in the `target-namespace` :
  ```shell
  kubectl -n target-namespace create secret generic github-personal-token \
          --from-literal token="TOKEN_AS_GENERATED_PREVIOUSLY"
  ```

    - And now create Repositry CRD with the secret field referencing it.

  Here is an example of a Repository CRD :

```yaml
---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: my-repo
  namespace: target-namespace
spec:
  url: "https://github.com/owner/repo"
  branch: "main"
  event_type: "pull_request"
  # Set this if you run with Github Enteprise
  # webvcs_api_url: "github.enteprise.com"
  webvcs_api_secret:
    name: "github-personal-token"
    # Set this if you have a different key in your secret
    # key: "token"
```

* Note that `webvcs_api_secret` cannot reference a secret in another namespace, Pipelines as code assumes always it will be
  the same namespace as where the repository has been created.

## Install Pipelines-As-Code for Bitbucket Cloud

Pipelines-As-Code has a full support on Bitbucket Cloud on <https://bitbucket.org>

* You have to first install the Pipelines-as-Code infrastructure as detailled
  here : [Install infrastructure](INSTALL.md#install-pipelines-as-code-infrastructure)

* You will have to generate an app password for Pipelines-as-Code Bitbucket API operations. Follow this guide to create
  an app password :

<https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/>

Make sure you note somewhere the generated token or otherwise you will have to recreate it.

* Go to you **"Repository setting"** tab on your **Repository** and click on the **WebHooks** tab and **"Add webhook"**
  button.

* Set a **Title** (i.e: Pipelines as Code)

* Set the URL to the event listenner public URL. On OpenShift you can get the public URL of the Pipelines-as-Code
  eventlistenner like this :

  ```shell
  echo https://$(oc get route -n pipelines-as-code el-pipelines-as-code-interceptor -o jsonpath='{.spec.host}')
  ```

* [Refer to this screenshot](./images/bitbucket-cloud-create-webhook.png) on how to configure the Webhook. The
  individual events to select are :
    * Repository -> Push
    * Pull Request -> Created
    * Pull Request -> Updated
    * Pull Request -> Comment created
    * Pull Request -> Comment updated


* You are now able to create a Repository CRD. The repository CRD will have a Secret and Username that contains the App
  Password as generated and Pipelines as Code will know how to use it for Bitbucket API operations.

    - First create the secret with the app password in the `target-namespace` :
  ```shell
  kubectl -n target-namespace create secret generic bitbucket-app-password \
          --from-literal token="TOKEN_AS_GENERATED_PREVIOUSLY"
  ```

    - And now create Repositry CRD with the secret field referencing it.

    - Here is an example of a Repository CRD :

```yaml
---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: my-repo
  namespace: target-namespace
spec:
  url: "https://bitbucket.com/workspace/repo"
  branch: "main"
  event_type: "pull_request"
  webvcs_api_user: "yourbitbucketusername"
  webvcs_api_secret:
    name: "github-token"
    # Set this if you have a different key in your secret
    # key: "token"
```

* Note that `webvcs_api_secret` cannot reference a secret in another namespace, Pipelines as code assumes always it will be
  the same namespace as where the repository has been created.

* There is no Webhook secret support in Bitbucket Cloud. To be able to secure the payload and not let a user hijack the
  CI, Pipelines-as-Code will fetch the ip addresses list from <https://ip-ranges.atlassian.com/> and make sure the
  webhook only comes from the Bitbucket Cloud IPS.
    * If you want to add some ips address or networks you can add them to the key
      **bitbucket-cloud-additional-source-ip** in the pipelines-as-code configmap in the pipelines-as-code namespace.
      You can added multiple network or ips separated by a comma.
    * If you want to disable this behaviour you can set the key **bitbucket-cloud-check-source-ip** to false in the
      pipelines-as-code configmap in the pipelines-as-code namespace.

## Pipelines-As-Code configuration settings.

There is a few things you can configure via the configmap `pipelines-as-code` in the `pipelines-as-code` namespace.

- `application-name`

  The name of the application showing for example in the GitHub Checks labels. Default to `"Pipelines as Code CI"`

- `max-keep-days`

  The number of the day to keep the PipelineRuns runs in the `pipelines-as-code` namespace. We install by default a
  cronjob that cleans up the PipelineRuns generated on events in pipelines-as-code namespace. Note that these
  PipelineRuns are internal to Pipelines-as-code are separate from the PipelineRuns that exist in the user's GitHub
  repository. The cronjob runs every hour and by default cleanups PipelineRuns over a day. This configmap setting
  doesn't affect the cleanups of the user's PipelineRuns which are controlled by
  the [annotations on the PipelineRun definition in the user's GitHub repository](#pipelineruns-cleanups).

- `secret-auto-create`

  Wether to auto create a secret with the token generated via the Github application to be used with private
  repositories. This feature is enabled by default.

## Kubernetes

Pipelines as Code should work directly on kubernetes/minikube/kind. You just need to install the release.yaml
for [pipeline](https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml)
, [triggers](https://storage.googleapis.com/tekton-releases/triggers/latest/release.yaml) and
its [interceptors](https://storage.googleapis.com/tekton-releases/triggers/latest/interceptors.yaml) on your cluster.
The release yaml to install pipelines are for the relesaed version :

```shell
VERSION=0.4.1
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-$VERSION/release-$VERSION.k8s.yaml
```

and for the nightly :

```shell
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-$VERSION/release.k8s.yaml
```

Kubernetes Dashboard is not yet supported for logs links but help is always welcome ;)

## CLI

`Pipelines as Code` provide a CLI which is design to work as tkn plugin. To install the plugin see the instruction
below.

### Binary releases

You can grab the latest binary directly from the
[releases](https://github.com/openshift-pipelines/pipelines-as-code/releases)
page.

### Dev release

If you want to install from the git repository you can just do :

```shell
go install github.com/openshift-pipelines/pipelines-as-code/cmd/tkn-pac
```

### Brew release

On [LinuxBrew](https://docs.brew.sh/Homebrew-on-Linux) or [OSX brew](https://brew.sh/) you can simply add the Brew tap
to have the tkn-pac plugin and its completion installed :

```shell
brew install openshift-pipelines/pipelines-as-code/tektoncd-pac
```

You simply need to do a :

```shell
brew upgrade openshift-pipelines/pipelines-as-code/tektoncd-pac
```

to upgrade to the latest released version.

### Container

`tkn-pac` is as well available inside the container image :

or from the container image user docker/podman:

```shell
docker run -e KUBECONFIG=/tmp/kube/config -v ${HOME}/.kube:/tmp/kube \
     -it quay.io/openshift-pipeline/pipelines-as-code tkn-pac help
```
