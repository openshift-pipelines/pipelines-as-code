---
title: GitHub Apps
weight: 10
---

# Create a Pipelines-as-Code GitHub App

The GitHub App install is different from the other install methods since it
acts as the integration point with OpenShift Pipelines and brings the Git
workflow into Tekton pipelines. You only need one GitHub App for every user on
the cluster usually setup by the admin.

You need the webhook of the GitHub App to point to your Pipelines-as-Code
Controller route or ingress endpoint which would listen to GitHub events.

There are 2 ways to set up GitHub App:

## Setup using tkn pac cli

You could use [`tkn pac bootstrap`](/docs/guide/cli) command which will a create GitHub App, provides
steps to configure it with your Git repository and also creates required secrets.
After creating the GitHub App, you must install it on the repositories you want to use for Pipelines-as-Code.

Alternatively, you could set up manually by following the steps [here](./#setup-manually)

## Manual SetUp

* Go to <https://github.com/settings/apps> (or *Settings > Developer settings > GitHub Apps*) and click on **New GitHub
  App** button
* Provide the following info in the GitHub App form
  * **GitHub Application Name**: `OpenShift Pipelines`
  * **Homepage URL**: *[OpenShift Console URL]*
  * **Webhook URL**: *[the Pipelines-as-Code route or ingress URL as copied in the previous section]*
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

* Subscribe to following events:
  * Check run
  * Check suite
  * Issue comment
  * Commit comment
  * Pull request
  * Push

{{< hint info >}}
> You can see a screenshot of how the GitHub App permissions look like [here](https://user-images.githubusercontent.com/98980/124132813-7e53f580-da81-11eb-9eb4-e4f1487cf7a0.png)
{{< /hint >}}

* Click on **Create GitHub App**.

* Take note of the **App ID** at the top of the page on the detail's page of the GitHub App you just created.

* In **Private keys** section, click on **Generate Private key* to generate a private key for the GitHub app. It will
  download automatically. Store the private key in a safe place as you need it in the next section and in future when
  reconfiguring this app to use a different cluster.

### Configure Pipelines-as-Code on your cluster to access the GitHub App

In order for Pipelines-as-Code to be able to authenticate to the GitHub App and have the GitHub App securely trigger the
Pipelines-as-Code webhook, you need to create a Kubernetes secret containing the private key of the GitHub App and the
webhook secret of the Pipelines-as-Code as it was provided when you created the GitHub App in the previous section. This
secret
is [used to generate](https://docs.github.com/en/developers/apps/building-github-apps/identifying-and-authorizing-users-for-github-apps)
a token on behalf of the user running the event and validating the webhook
through the webhook secret.

Run the following command and replace:

* `APP_ID` with the GitHub App **App ID** copied in the previous section
* `WEBHOOK_SECRET` with the webhook secret provided when created the GitHub App
  in the previous section
* `PATH_PRIVATE_KEY` with the path to the private key that was downloaded in the
  previous section

```bash
kubectl -n pipelines-as-code create secret generic pipelines-as-code-secret \
        --from-literal github-private-key="$(cat $PATH_PRIVATE_KEY)" \
        --from-literal github-application-id="APP_ID" \
        --from-literal webhook.secret="WEBHOOK_SECRET"
```

Lastly, install the App on any repos you'd like to use with Pipelines-as-Code.

## GitHub Enterprise

Pipelines-as-Code supports GitHub Enterprise.

You don't need to do anything special to get Pipelines as code working with
GHE. Pipelines as code automatically detect the header as set from GHE and
use the GHE API auth URL rather than the public GitHub.
