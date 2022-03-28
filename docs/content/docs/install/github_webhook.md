---
title: Github Webhook
weight: 2
---

# Install Pipelines-as-Code as a GitHub Webhook

If you are not able to create a GitHub application you can install Pipelines-as-Code on your repository as a
[GitHub Webhook](https://docs.github.com/en/developers/webhooks-and-events/webhooks/creating-webhooks).

Using Pipelines as Code via Github webhook does not give you access to the GitHub CheckRun API, therefore the status of
the tasks will be added as a Comment of the PR and not via the **Checks** Tab.

Following the [infrastructure installation](install.md#install-pipelines-as-code-infrastructure)

* You will have to generate a personal token for Pipelines-as-Code Github API operations.

  Follow this guide to create a personal token :

<https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token>

  The only permission needed is the *repo* permission. Make sure you note somewhere the generated token or otherwise you
  will have to recreate it.

* Go to you repository or organization setting and click on *Hooks* and *"Add webhook"* links.

* Set the payload URL to the event listener public URL. On OpenShift you can get the public URL of the
  Pipelines-as-Code controller like this :

  ```shell
  echo https://$(oc get route -n pipelines-as-code el-pipelines-as-code-interceptor -o jsonpath='{.spec.host}')
  ```

* Add a secret or generate a random one with this command  :

  ```shell
  openssl rand -hex 20
  ```

* You will need to create a webhook on your repository. The individual events for the webhook to select are :
  * Commit comments
  * Issue comments
  * Pull request reviews
  * Pull request
  * Pushes

{{< hint info >}}
[Refer to this screenshot](/images/pac-direct-webhook-create.png) on how to configure the Webhook.
{{< /hint >}}

* On your cluster you need create the webhook secret as generated previously in the *pipelines-as-code* namespace.

```shell
kubectl -n pipelines-as-code create secret generic pipelines-as-code-secret \
        --from-literal webhook.secret="$WEBHOOK_SECRET_AS_GENERATED"
```

* You are now able to create a Repository CRD. The repository CRD will have a
  Secret that contains the Personal token as generated and Pipelines as Code
  will know how to use it for GitHub API operations.

* First create the secret with the personal token in the `target-namespace` :

  ```shell
  kubectl -n target-namespace create secret generic github-personal-token \
          --from-literal token="TOKEN_AS_GENERATED_PREVIOUSLY"
  ```

* And now create Repository CRD with the secret field referencing it.

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
  git_provider:
    url: "https://github.enterprise.com"
    secret:
      name: "github-personal-token"
      # Set this if you have a different key in your secret
      # key: "token"
```

* Note that `git_provider.secret` cannot reference a secret in another
  namespace, Pipelines as code assumes always it will be the same namespace as
  where the repository has been created.
