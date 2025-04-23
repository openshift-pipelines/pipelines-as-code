---
title: GitLab
weight: 13
---

# Use Pipelines-as-Code with GitLab Webhook

Pipelines-As-Code supports on [GitLab](https://www.gitlab.com) through a webhook.

Follow the pipelines-as-code [installation](/docs/install/installation) according to your Kubernetes cluster.

## Create GitLab Personal Access Token

* Follow this guide to generate a personal token as the manager of the Org or the Project:

  <https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html>

  **Note**: You can create a token scoped only to the project. Since the
  token needs to be able to have `api` access to the forked repository from where
  the MR come from, it will fail to do it with a project scoped token. We try
  to fallback nicely by showing the status of the pipeline directly as comment
  of the Merge Request.

## Create a `Repository` and configure webhook

There are two ways to create the `Repository` and configure the webhook:

### Create a `Repository` and configure webhook using the `tkn pac` tool

* Use the [`tkn pac create repo`](/docs/guide/cli) command to
configure a webhook and create the `Repository` CR.

  You need to have a personal access token created with `api` scope. `tkn pac` will use this token to configure the webhook, and add it in a secret
in the cluster which will be used by Pipelines-As-Code controller for accessing the `Repository`.

Below is the sample format for `tkn pac create repo`

```shell script
$ tkn pac create repo

? Enter the Git repository url (default: https://gitlab.com/repositories/project):
? Please enter the namespace where the pipeline should run (default: project-pipelines):
! Namespace project-pipelines is not found
? Would you like me to create the namespace project-pipelines? Yes
✓ Repository repositories-project has been created in project-pipelines namespace
✓ Setting up GitLab Webhook for Repository https://gitlab.com/repositories/project
? Please enter the project ID for the repository you want to be configured,
  project ID refers to an unique ID (e.g. 34405323) shown at the top of your GitLab project : 17103
👀 I have detected a controller url: https://pipelines-as-code-controller-openshift-pipelines.apps.awscl2.aws.ospqa.com
? Do you want me to use it? Yes
? Please enter the secret to configure the webhook for payload validation (default: lFjHIEcaGFlF):  lFjHIEcaGFlF
ℹ ️You now need to create a GitLab personal access token with `api` scope
ℹ ️Go to this URL to generate one https://gitlab.com/-/profile/personal_access_tokens, see https://is.gd/rOEo9B for documentation
? Please enter the GitLab access token:  **************************
? Please enter your GitLab API URL:  https://gitlab.com
✓ Webhook has been created on your repository
🔑 Webhook Secret repositories-project has been created in the project-pipelines namespace.
🔑 Repository CR repositories-project has been updated with webhook secret in the project-pipelines namespace
ℹ Directory .tekton has been created.
✓ A basic template has been created in /home/Go/src/gitlab.com/repositories/project/.tekton/pipelinerun.yaml, feel free to customize it.
ℹ You can test your pipeline by pushing the generated template to your git repository
```

### Create a `Repository` and configure webhook manually

* From the left navigation pane of your GitLab repository, go to **settings** -->
  **Webhooks** tab.

* Go to your project and click on *Settings* and *"Webhooks"* from the sidebar on the left.

  * Set the **URL** to Pipelines-as-Code controller public URL. On OpenShift, you can get the public URL of the
  Pipelines-as-Code controller like this :

    ```shell
    echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
    ```

  * Add a secret or generate a random one with this command  :

    ```shell
    head -c 30 /dev/random | base64
    ```

  * [Refer to this screenshot](/images/gitlab-add-webhook.png) on how to configure the Webhook.

    The individual  events to select are :

    * Merge request Events
    * Push Events
    * Comments
    * Tag push events

  * Click on **Add webhook**

* You can now create a [`Repository CRD`](/docs/guide/repositorycrd).
  It will have:

  * A reference to a Kubernetes **Secret** containing the Personal token and
  another reference to a Kubernetes secret to validate the Webhook payload as set previously in your Webhook configuration.

* Create the secret with the personal token and webhook secret in the `target-namespace` (where you are planning to run your pipeline CI):

  ```shell
  kubectl -n target-namespace create secret generic gitlab-webhook-config \
    --from-literal provider.token="TOKEN_AS_GENERATED_PREVIOUSLY" \
    --from-literal webhook.secret="SECRET_AS_SET_IN_WEBHOOK_CONFIGURATION"
  ```

* Create the `Repository` CRD with the secret field referencing it. For example:

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
      # url: "https://gitlab.example.com/ # Set this if you are using a private GitLab instance
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

* Private instances are not automatically detected for GitLab yet, so you will need to specify the API URL under the spec `git_provider.url`.

* If you want to override the API URL then you can simply add it to the spec`.git_provider.url` field.

* The `git_provider.secret` key cannot reference to a secret in another namespace.
  Pipelines as code always assumes that it will be in the same namespace where the
  `Repository` has been created.

## Add webhook secret

* For an existing `Repository`, if webhook secret has been deleted (or you want to add a new webhook to project settings) for Bitbucket Cloud,
  use `tkn pac webhook add` command to add a webhook to project repository settings, as well as update the `webhook.secret`
  key in the existing `Secret` object without updating `Repository`.

Below is the sample format for `tkn pac webhook add`

```shell script
$ tkn pac webhook add -n project-pipelines

✓ Setting up GitLab Webhook for Repository https://gitlab.com/repositories/project
? Please enter the project ID for the repository you want to be configured,
  project ID refers to an unique ID (e.g. 34405323) shown at the top of your GitLab project : 17103
👀 I have detected a controller url: https://pipelines-as-code-controller-openshift-pipelines.apps.awscl2.aws.ospqa.com
? Do you want me to use it? Yes
? Please enter the secret to configure the webhook for payload validation (default: TXArbGNDHTXU):  TXArbGNDHTXU
✓ Webhook has been created on your repository
🔑 Secret repositories-project has been updated with webhook secret in the project-pipelines namespace.

```

**Note:** If `Repository` exist in a namespace other than the `default` namespace, use `tkn pac webhook add [-n namespace]`.
  In the above example, `Repository` exist in the `project-pipelines` namespace rather than the `default` namespace; therefore
  the webhook was added in the `project-pipelines` namespace.

## Update token

There are two ways to update the provider token for the existing `Repository`:

### Update using tkn pac CLI

* Use the [`tkn pac webhook update-token`](/docs/guide/cli) command which
  will update provider token for the existing Repository CR.

Below is the sample format for `tkn pac webhook update-token`

```shell script
$ tkn pac webhook update-token -n repo-pipelines

? Please enter your personal access token:  **************************
🔑 Secret repositories-project has been updated with new personal access token in the project-pipelines namespace.
```

**Note:** If `Repository` exist in a namespace other than the `default` namespace, use `tkn pac webhook add [-n namespace]`.
  In the above example, `Repository` exist in the `project-pipelines` namespace rather than the `default` namespace; therefore
  the webhook was added in the `project-pipelines` namespace.

### Update by changing `Repository` YAML or using `kubectl patch` command

When you have regenerated a new token, you must update it in the cluster.
For example, you can replace `$NEW_TOKEN` and `$target_namespace` with their respective values:

You can find the secret name in the `Repository` CR.

  ```yaml
  spec:
    git_provider:
      # url: "https://gitlab.example.com/ # Set this if you are using a private GitLab instance
      secret:
        name: "gitlab-webhook-config"
  ```

```shell
kubectl -n $target_namespace patch secret gitlab-webhook-config -p "{\"data\": {\"provider.token\": \"$(echo -n $NEW_TOKEN|base64 -w0)\"}}"
```
