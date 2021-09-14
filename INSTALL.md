# Installation Guides

In order to install and use Pipelines-as-Code, you need to 
* Install the Pipeline-as-Code infrastructure on your cluster 
* Create a Pipeline-as-Code GitHub App on your GitHub account or organization
* Configure Pipeline-as-Code on your cluster to access the GitHub App

Here is a video walkthrough of the install process :

[![Pipelines as Code Install Walkthought](https://img.youtube.com/vi/d81rIHNFjJM/0.jpg)](https://www.youtube.com/watch?v=d81rIHNFjJM)

## Install Pipelines as Code infrastructure

To install Pipelines as Code on your cluster you simply need to run this command :

```shell
VERSION=0.2
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-$VERSION/release-$VERSION.yaml
```

If you would like to install the current development version you can simply install it like this :

```shell
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/nightly/release.yaml
```

It will apply the release.yaml to your kubernetes cluster, creating the
admin namespace `pipelines-as-code`, the roles and all other bits needed.

The `pipelines-as-code` namespace is where the pipeline-as-code infrastructure runs and is supposed to be accessible only by the admin.

The Pipeline-as-Code EventListener requires an OpenShift route to be accessible from GitHub. Run the following to create a route:

```
oc expose service el-pipelines-as-code-interceptor -n pipelines-as-code
```

Enable TLS on the Pipeline-as-Code EventListener:

```
oc apply -n pipelines-as-code -f <(oc get -n pipelines-as-code route el-pipelines-as-code-interceptor  -o json |jq -r '.spec |= . + {tls: {"insecureEdgeTerminationPolicy": "Redirect", "termination": "edge"}}')
```

Retrieve the EventListener URL which you will need in the next section when creating the GitHub App:
```
echo https://$(oc get route -n pipelines-as-code el-pipelines-as-code-interceptor -o jsonpath='{.spec.host}')
```

## Create a Pipeline-as-Code GitHub App

You should now create a Pipeline-as-Code GitHub App which acts as the integration point with OpenShift Pipelines and brings the Git workflow into Tekton pipelines. You need the webhook of the GitHub App pointing to your Pipeline-as-Code EventListener route endpoint which would then trigger pipelines on GitHub events.

* Go to https://github.com/settings/apps (or *Settings > Developer settings > GitHub Apps*) and click on **New GitHub App** button
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

* In **Private keys** section, click on **Generate Private key* to generate a private key for the GitHub app. It will download automatically. Store the private key in a safe place as you need it in the next section and in future when reconfiguring this app to use a different cluster.

## Configure Pipeline-as-Code on your cluster to access the GitHub App

In order for Pipeline-as-Code to be able to authenticate to the GitHub App and the GitHub App securely trigger the Pipeline-as-Code webhook, you need to create a Kubernetes secret containing the private key of the GitHub App and the webhook secret of the Pipeline-as-Code as it was provided when you created the GitHub App in the previous section. This secret is used to generate a token on behalf of the user running the event and make sure to validate the webhook via the webhook secret.

Run the following command and replace:
* `APP_ID` with the GitHub App **App ID** copied in the previous section
* `WEBHOOK_SECRET` with the webhook secret provided when created the GitHub App in the previous section
* `PATH_PRIVATE_KEY` with the path to the private key that was downloaded in the previous section

```bash
kubectl -n pipelines-as-code create secret generic github-app-secret \
        --from-literal private.key="$(cat PATH_PRIVATE_KEY)"
        --from-literal application_id="APP_ID" \
        --from-literal webhook.secret="WEBHOOK_SECRET"
```

## GitHub Enterprise

Pipelines as Code supports Github Enterprise.

You don't need to do anything special to get Pipelines as code working with GHE.
Pipelines as code will automatically detects the header as set from GHE and use it  the GHE API auth url instead of the public github.
