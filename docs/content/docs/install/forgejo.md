---
title: Forgejo
weight: 14
---

{{< tech_preview "Forgejo" >}}

# Use Pipelines-as-Code with Forgejo Webhook

Pipelines-as-Code supports [Forgejo](https://forgejo.org) through a webhook.

Forgejo is a community-driven Git forge that originated as a fork of Gitea. Pipelines-as-Code originally supported Gitea and now supports Forgejo, maintaining API compatibility between the two platforms. Both use the same provider type (`gitea`) in Pipelines-as-Code configuration.

Follow the Pipelines-as-Code [installation](/docs/install/installation) according to your Kubernetes cluster.

## Create Forgejo Personal Access Token

Create a Forgejo token for Pipelines-as-Code by going to the Applications tab
of the user settings, or to this URL (replace the domain name with your domain
name).

<https://your.forgejo.domain/user/settings/applications>

When creating the token, select these scopes:

### Required Scopes

These scopes are necessary for basic Pipelines-as-Code functionality:

- **Repository** (Write) - For setting commit status and reading repository contents
- **Issue** (Write) - For creating and editing comments on pull requests

### Optional Scopes

- **Organization** (Read) - Only required if using [team-based policies]({{< relref "/docs/guide/policy" >}}) to restrict pipeline triggers based on Forgejo organization team membership

{{< hint info >}}
For most users, only the **Required Scopes** are needed. Skip Organization (Read) unless you plan to use `policy.team_ids` in your Repository CRD configuration.
{{< /hint >}}

Keep the generated token noted somewhere, or otherwise you will have to recreate it.

## Create a `Repository` and configure webhook

{{< hint info >}}
The `tkn pac create repo` and `tkn pac webhook` commands do not currently support Forgejo. You must configure the webhook manually.
{{< /hint >}}

### Configure webhook manually

1. From your Forgejo repository, go to **Settings** -> **Webhooks** and click **Add Webhook** -> **Forgejo**.

2. Set the **HTTP method** to **POST** and **POST content type** to **application/json**.

3. Set the **Target URL** to the Pipelines-as-Code controller public URL. On OpenShift, you can get the public URL like this:

   ```shell
   echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
   ```

   _If you are not using OpenShift you will need to get the public route from your ingress controller._

4. Set a **Secret** or generate a random one with:

   ```shell
   head -c 30 /dev/random | base64
   ```

5. Select the following **Trigger On** events under **Custom events...** (these map to the events Pipelines-as-Code processes):

   **Repository events:**
   - Push

   **Pull request events:**
   - Opened
   - Reopened
   - Synchronized
   - Label updated
   - Closed

   **Issue events:**
   - Comments (only comments on open pull requests are processed)

6. Click **Add Webhook**.

### Create the Secret

Create a secret with the personal token and webhook secret in your target namespace:

```shell
kubectl -n target-namespace create secret generic forgejo-webhook-config \
  --from-literal provider.token="TOKEN_AS_GENERATED_PREVIOUSLY" \
  --from-literal webhook.secret="SECRET_AS_SET_IN_WEBHOOK_CONFIGURATION"
```

If you configured an empty webhook secret, use an empty string:

```shell
kubectl -n target-namespace create secret generic forgejo-webhook-config \
  --from-literal provider.token="TOKEN_AS_GENERATED_PREVIOUSLY" \
  --from-literal webhook.secret=""
```

### Create the Repository CRD

Create a [`Repository CRD`](/docs/guide/repositorycrd) with the secret field referencing it:

```yaml
---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: my-repo
  namespace: target-namespace
spec:
  url: "https://forgejo.example.com/owner/repo"
  git_provider:
    # Use "gitea" as the type - Forgejo is API-compatible with Gitea
    type: "gitea"
    # Set this to your Forgejo instance URL
    url: "https://forgejo.example.com"
    secret:
      name: "forgejo-webhook-config"
      # Set this if you have a different key in your secret
      # key: "provider.token"
    webhook_secret:
      name: "forgejo-webhook-config"
      # Set this if you have a different key in your secret
      # key: "webhook.secret"
```

## Notes

- **Provider Type**: Use `type: "gitea"` in your Repository CRD. Forgejo is a fork of Gitea and maintains full API compatibility.

- **Forgejo Instance URL**: You must specify `git_provider.url` pointing to your Forgejo instance URL.

- **Webhook Secret**: Pipelines-as-Code currently does not validate webhook signatures for Forgejo/Gitea. Secrets can be stored, but requests are accepted without signature verification.

- The `git_provider.secret` key cannot reference a secret in another namespace. Pipelines-as-Code always assumes it will be in the same namespace where the `Repository` has been created.

## Update Token

When you have regenerated a new token, you must update it in the cluster. You can find the secret name in the `Repository` CR:

```yaml
spec:
  git_provider:
    secret:
      name: "forgejo-webhook-config"
```

Update the secret:

```shell
kubectl -n target_namespace patch secret forgejo-webhook-config -p "{\"data\": {\"provider.token\": \"$(echo -n $NEW_TOKEN|base64 -w0)\"}}"
```
