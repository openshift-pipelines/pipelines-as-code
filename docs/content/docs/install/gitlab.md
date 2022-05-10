---
title: Gitlab
weight: 13
---

# Install Pipelines-as-Code for Gitlab

Pipelines-As-Code supports [Gitlab](https://www.gitlab.com) through a webhook.

Follow the pipelines-as-code [installation](/docs/install/installation) according to your kubernetes cluster.

## Generate a token for Pipelines as Code

* You will have to generate a personal token as the manager of the Org or the Project,
  follow the steps here :

  <https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html>

  **Note**: You can create a token scoped only to the project. Since the
  token needs to be able to have `api` access to the forked repository from where
  the MR come from, it will fail to do it with a project scoped token. We try
  to fallback nicely by showing the status of the pipeline directly as comment
  of the Merge Request.

{{< hint danger >}}
For best security practice you will probably want to have a short token
expiration (like the default 30 days). Gitlab will send you a notification email
if you token expires. When you have regenerated a new token you will need to
update it on cluster. For example through the command line, you will want to replace
`$NEW_TOKEN` and `$target_namespace` by their respective values:

```shell
kubectl -n $target_namespace patch secret gitlabwebhook -p "{\"data\": {\"foo\": \"$(echo -n $NEW_TOKEN|base64 -w0)\"}}"
```

{{< /hint >}}

## Repository creation

Now, you have 2 ways to configure the webhook:

* You could use [`tkn pac repository create`](/docs/guide/cli) command which
  will create repository CR and configure webhook, or
* You could follow the [configuring webhook](#configure-webhook) guide to do it manually

## Configure webhook

* Go to your project and click on *Settings* and *"Webhooks"* from the sidebar on the left.

* Set the payload URL to the event listener public URL. On OpenShift, you can get the public URL of the
  Pipelines-as-Code controller like this :

  ```shell
  echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
  ```

* Add a secret or generate a random one with this command  :

  ```shell
  openssl rand -hex 20
  ```

* [Refer to this screenshot](/images/gitlab-add-webhook.png) on how to configure the Webhook.

  The individual  events to select are :

  * Merge request Events
  * Push Events
  * Note Events

* You are now able to create a Repository CRD. The repository CRD will reference a Kubernetes Secret containing the Personal token
and another reference to a Kubernetes secret to validate the Webhook payload as set previously in your Webhook configuration.

* First create the secret with the personal token and webhook secret in the `target-namespace` (where you are planning to run your pipeline CI) :

  ```shell
  kubectl -n target-namespace create secret generic gitlab-webhook-config \
    --from-literal provider.token="TOKEN_AS_GENERATED_PREVIOUSLY" \
    --from-literal webhook.secret="SECRET_AS_SET_IN_WEBHOOK_CONFIGURATION"
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
    url: "https://gitlab.com/group/project"
    git_provider:
      secret:
        name: "gitlab-webhook-config"
        # Set this if you have a different key in your secret
        # key: "provider.token"
      webhook_secret:
        name: "gitlab-webhook-config"
        # Set this if you have a different key in your secret
        # key: "webhook.secret"
  ```

## Notes

* Private instance are automatically detected, no need to specify the api URL. Unless you want to override it then you can simply add it to the spec`.git_provider.url` field.

* `git_provider.secret` cannot reference a secret in another namespace,
  Pipelines as code always assumes it will be the same namespace as where the
  repository has been created.
