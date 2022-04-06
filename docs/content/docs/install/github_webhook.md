---
title: GitHub Webhook
weight: 12
---

# Install Pipelines-as-Code as a GitHub Webhook

If you are not able to create a GitHub application you can install Pipelines-as-Code on your repository as a
[GitHub Webhook](https://docs.github.com/en/developers/webhooks-and-events/webhooks/creating-webhooks).

Using Pipelines as Code via GitHub webhook does not give you access to the [GitHub CheckRun API](https://docs.github.com/en/rest/guides/getting-started-with-the-checks-api), therefore the status of
the tasks will be added as a Comment of the PR and not via the **Checks** Tab.

After you have finished the [infrastructure installation](install.md#install-pipelines-as-code-infrastructure) you can generate an app password for Pipelines-as-Code GitHub API operations.

Follow this guide to create a personal token :

<https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token>

The only permission needed is the *repo* permission. Make sure you note the generated token somewhere or otherwise you will have to recreate it.

* Go to you repository or organization setting and click on *Hooks* and *"Add webhook"* links.

* Set the payload URL to Pipeline as Code public URL. On OpenShift you can get the public URL of the Pipelines-as-Code controller like this:

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
  * Pull request reviews
  * Pull request
  * Pushes

{{< hint info >}}
[Refer to this screenshot](../../../images/pac-direct-webhook-create.png) to make sure  you have properly configured the webhook.
{{< /hint >}}

* You are now able to create a Repository CRD. The repository CRD will reference a
  Kubernetes Secret containing the Personal token as generated previously and another reference to a Kubernetes secret to validate the Webhook payload as set previously in your Webhook configuration .

* First create the secret with the personal token in the `target-namespace` :

  ```shell
  kubectl -n target-namespace create secret generic github-personal-token \
          --from-literal token="TOKEN_AS_GENERATED_PREVIOUSLY"
  ```

* Then create the secret with the secret name as set in the Webhook configuration :

  ```shell
  kubectl -n target-namespace create secret generic github-webhook-secret \
          --from-literal secret="SECRET_NAME_AS_SET_IN_WEBHOOK_CONFIGURATION"
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
      name: "github-personal-token"
      # Set this if you have a different key in your secret
      # key: "token"
    webhook_secret:
      name: "github-webhook-secret"
      # Set this if you have a different key for your secret
      # key: "secret-name"
```

## GitHub webhook Notes

* Secrets needs to be in the same namespace as installed on Repository, they cannot be on another namespace.
