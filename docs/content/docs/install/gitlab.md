---
title: Gitlab
weight: 3
---

# Install Pipelines-as-Code for Gitlab

Pipelines-As-Code supports [Gitlab](https://www.gitlab.com) via a webhook.

Following the [infrastructure installation](install.md#install-pipelines-as-code-infrastructure):

* You will have to generate a personal token as the manager of the Org or the Project,
  follow the steps here :

  <https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html>

  **Note**: You can create a token scoped only to the project. Since the
  token needs to be able to have `api` access to the forked repository from where
  the MR come from, it will fail to do it with a project scoped token. We try
  to fallback nicely by showing the status of the pipeline directly as comment
  of the the Merge Request.

* Go to your project and click on *Settings* and *"Webhooks"* from the sidebar on the left.

* Set the payload URL to the event listener public URL. On OpenShift you can get the public URL of the
  Pipelines-as-Code controller like this :

  ```shell
  echo https://$(oc get route -n pipelines-as-code el-pipelines-as-code-interceptor -o jsonpath='{.spec.host}')
  ```

* Add a secret or generate a random one with this command  :

  ```shell
  openssl rand -hex 20
  ``

* [Refer to this screenshot](/images/gitlab-add-webhook.png) on how to configure the Webhook.

  The individual  events to select are :

  * Merge request Events
  * Push Events
  * Note Events

* On your cluster you need create the webhook secret as generated previously in the *pipelines-as-code* namespace.

```shell
kubectl -n pipelines-as-code create secret generic pipelines-as-code-secret \
        --from-literal webhook.secret="$WEBHOOK_SECRET_AS_GENERATED"
```

* You are now able to create a Repository CRD. The repository CRD will have a
  Secret that contains the Personal token as generated and Pipelines as Code
  will know how to use it for Gitlab API operations.

* First create the secret with the personal token in the `target-namespace` (where you are planning to run your pipeline CI) :

  ```shell
  kubectl -n target-namespace create secret generic gitlab-personal-token \
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
  url: "https://gitlab.com/group/project"
  git_provider:
    secret:
      name: "gitlab-personal-token"
      # Set this if you have a different key in your secret
      # key: "token"
```

## Notes

* Private instance are automatically detected, no need to specify the api url. Unless you want to override it then you can simply add it to the spec`.git_provider.url` field.

* `git_provider.secret` cannot reference a secret in another namespace,
  Pipelines as code assumes always it will be the same namespace as where the
  repository has been created.
