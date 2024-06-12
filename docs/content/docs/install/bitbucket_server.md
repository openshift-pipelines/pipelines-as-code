---
title: Bitbucket Server
weight: 15
---
# Install Pipelines-As-Code on Bitbucket Server

Pipelines-As-Code has a full support of [Bitbucket
Server](https://www.atlassian.com/software/bitbucket/enterprise).

After following the [installation](/docs/install/installation):

* You will have to generate a personal token as the manager of the Project,
  follow the steps here:

<https://confluence.atlassian.com/bitbucketserver/personal-access-tokens-939515499.html>

The token will need to have the `PROJECT_ADMIN` and `REPOSITORY_ADMIN` permissions.

Note that the token needs to be able to have access to the forked repository in
pull requests, or it would not be able to process and access the pull request.

You may want to note somewhere the generated token, or otherwise you will have to
recreate it.

* Create a Webhook on the repository following this guide :

<https://support.atlassian.com/bitbucket-cloud/docs/manage-webhooks/>

* Add a Secret or generate a random one with :

```shell
  head -c 30 /dev/random | base64
```

* Set the payload URL to Pipelines-as-Code public URL. On OpenShift, you can get the
  public URL of the Pipelines-as-Code route like this :

  ```shell
  echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
  ```

* [Refer to this screenshot](/images/bitbucket-server-create-webhook.png) on
  which events to handle on the Webhook. The individual events to select are :

  * Repository -> Push
  * Repository -> Modified
  * Pull Request -> Opened
  * Pull Request -> Source branch updated
  * Pull Request -> Comments added

  * Create a secret with personal token in the `target-namespace`

  ```shell
  kubectl -n target-namespace create secret generic bitbucket-server-webhook-config \
    --from-literal provider.token="TOKEN_AS_GENERATED_PREVIOUSLY" \
    --from-literal webhook.secret="SECRET_AS_SET_IN_WEBHOOK_CONFIGURATION"
  ```

* And finally create Repository CRD with the secret field referencing it.

  * Here is an example of a Repository CRD :

```yaml
  ---
  apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
  kind: Repository
  metadata:
    name: my-repo
    namespace: target-namespace
  spec:
    url: "https://bitbucket.com/workspace/repo"
    git_provider:
      # make sure you have the right bitbucket server api url without the
      # /api/v1.0 usually the # default install will have a /rest suffix
      url: "https://bitbucket.server.api.url/rest"
      user: "your-bitbucket-username"
      secret:
        name: "bitbucket-server-webhook-config"
        # Set this if you have a different key in your secret
        # key: "provider.token"
      webhook_secret:
        name: "bitbucket-server-webhook-config"
        # Set this if you have a different key for your secret
        # key: "webhook.secret"
```

## Notes

* `git_provider.secret` cannot reference a secret in another namespace,
  Pipelines as code always assumes it will be the same namespace as where the
  repository has been created.

* `tkn-pac create` and `bootstrap` is not supported on Bitbucket Server.

{{< hint danger >}}

* You can only reference user by the `ACCOUNT_ID` in owner file.

{{< /hint >}}
