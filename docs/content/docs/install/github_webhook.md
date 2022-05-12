---
title: GitHub Webhook
weight: 12
---

# Install Pipelines-as-Code as a GitHub Webhook

If you are not able to create a GitHub application you can install Pipelines-as-Code on your repository as a
[GitHub Webhook](https://docs.github.com/en/developers/webhooks-and-events/webhooks/creating-webhooks).

Using Pipelines as Code through GitHub webhook does not give you access to the
[GitHub CheckRun
API](https://docs.github.com/en/rest/guides/getting-started-with-the-checks-api),
therefore the status of
the tasks will be added as a Comment of the PR and not through the **Checks** Tab.

After you have finished the [installation](/docs/install/installation) you can
generate an app password for Pipelines-as-Code GitHub API operations.

## Generate a token for Pipelines as Code

Follow this guide to create a personal token :

<https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token>

Depending on the Repository access scope, the token will need different permissions.
For public repositories the scope are:

* `public_repo` scope

For private repositories:

* The whole `repo` scope

You will have to note the generated token somewhere, or otherwise you will have to recreate it.

NOTE: If you are going to configure webhook through CLI, you will need to also add a scope `admin:repo_hook`.

{{< hint danger >}}
For best security practice you will probably want to have a short token
expiration (like the rdefault 30 days). GitHub will send you a notification email
if you token expires. When you have regenerated a new token you will need to
update it on cluster. For example through the command line, you will want to replace
`$NEW_TOKEN` and `$target_namespace` by their respective values:

```shell
kubectl -n $target_namespace patch secret githubwebhook -p "{\"data\": {\"foo\": \"$(echo -n $NEW_TOKEN|base64 -w0)\"}}"
```

{{< /hint >}}

## Creating a Repository CR and Configuring webhook

Now, you have 2 ways to configure the webhook:

You could use [`tkn pac repository create`](/docs/guide/cli) command which
  will create repository CR and configure webhook

  You need to have a personal access token created with `admin:repo_hook` scope. tkn-pac will use this token to configure the
webhook and add it in a secret on cluster which will be used by controller for accessing the repository.
After configuring the webhook, you will be able to update the token in the secret with just the scopes mentioned [here](#generate-a-token-for-pipelines-as-code).

Alternatively, you could follow the [Configure webhook manually](#configure-webhook-manually) guide to do it manually

## Configuring webhook manually

follow below instruction to configure webhook manually

* Go to you repository or organization setting and click on *Hooks* and *“Add webhook“* links.

* Set the payload URL to Pipeline as Code public URL. On OpenShift, you can get the public URL of the Pipelines-as-Code controller like this:

  ```shell
  echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
  ```

* Add a Webhook secret or generate a random one with this command (and note it, we will need it later):

  ```shell
  openssl rand -hex 20
  ```

* Click "Let me select individual events" and select these events:
  * Commit comments
  * Issue comments
  * Pull request
  * Pushes

{{< hint info >}}
[Refer to this screenshot](/images/pac-direct-webhook-create.png) to verify you have properly configured the webhook.
{{< /hint >}}

* You are now able to create a Repository CRD. The repository CRD will reference a
  Kubernetes Secret containing the Personal token as generated previously and another reference to a Kubernetes secret to validate the Webhook payload as set previously in your Webhook configuration .

* First create the secret with the personal token and webhook secret in the `target-namespace` :

  ```shell
  kubectl -n target-namespace create secret generic github-webhook-config \
    --from-literal provider.token="TOKEN_AS_GENERATED_PREVIOUSLY" \
    --from-literal webhook.secret="SECRET_AS_SET_IN_WEBHOOK_CONFIGURATION"
  ```

* And now create Repository CRD referencing everything :

  ```yaml
  ---
  apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
  kind: Repository
  metadata:
    name: my-repo
    namespace: target-namespace
  spec:
    url: "https://github.com/owner/repo"
    git_provider:
      secret:
        name: "github-webhook-config"
        # Set this if you have a different key in your secret
        # key: "provider.token"
      webhook_secret:
        name: "github-webhook-config"
        # Set this if you have a different key for your secret
        # key: "webhook.secret"
  ```

## GitHub webhook Notes

* Secrets need to be in the same namespace as installed on Repository, they cannot be on another namespace.
