# Install Pipelines-As-Code for Bitbucket Server

Pipelines-As-Code has a full support of [Bitbucket
Server](https://www.atlassian.com/software/bitbucket/enterprise).

Following the [infrastructure installation](install.md#install-pipelines-as-code-infrastructure) :

* You will have to generate a personal token as the manager of the Project,
  follow the steps here :

<https://confluence.atlassian.com/bitbucketserver/personal-access-tokens-939515499.html>

The token will need to have the `PROJECT_ADMIN` and `REPOSITORY_ADMIN` permissions.

Note that the token needs to be able to have access to the forked repository in
pull requests or it would not be able to process and access the pull request.

Make sure you note somewhere the generated token or otherwise you will have to
recreate it.

* Create a Webhook on the repository following this guide :

<https://support.atlassian.com/bitbucket-cloud/docs/manage-webhooks/>

* Add a Secret or generate a random one with :

```shell
  openssl rand -hex 20
```

* Set the URL to the event listener public URL. On OpenShift you can get the
  public URL of the Pipelines-as-Code route like this :

  ```shell
  echo https://$(oc get route -n pipelines-as-code el-pipelines-as-code-interceptor -o jsonpath='{.spec.host}')
  ```

* Install the secret in the pipelines-as-code namespace (we currently only
supports one webhook secret per cluster ) :

```shell
kubectl -n pipelines-as-code create secret generic pipelines-as-code-secret \
        --from-literal webhook.secret="$WEBHOOK_SECRET_AS_GENERATED"
```

* [Refer to this screenshot](/images/bitbucket-server-create-webhook.png) on
  which events to handle on the Webhook. The individual events to select are :

  * Repository -> Push
  * Repository -> Modified
  * Pull Request -> Opened
  * Pull Request -> Source branch updated
  * Pull Request -> Comments added

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
    url: "https://bitbucket.server.api.url"
    user: "yourbitbucketusername"
    secret:
      name: "bitbucket-server-token"
      # Set this if you have a different key in your secret
      # key: "token"
```

## Notes

* `git_provider.secret` cannot reference a secret in another namespace,
  Pipelines as code assumes always it will be the same namespace as where the
  repository has been created.

* `tkn-pac create` and `bootstrap` is not supported on Bitbucket Server.

{{< hint danger >}}
* You can only reference user by the `ACCOUNT_ID` in owner file.
{{< /hint >}}
