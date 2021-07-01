# Install

Here is a video walkthought explaining going thought the install process :

[![Pipelines as Code Install Walkthought](https://img.youtube.com/vi/d81rIHNFjJM/0.jpg)](https://www.youtube.com/watch?v=d81rIHNFjJM)

## Pipelines as Code Install

To install Pipelines as Code on your server you simply need to run this command :

```shell
VERSION=0.1
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-$VERSION/release-$VERSION.yaml
```

If you would like to install the current developement version you can simply install it like this :

```shell
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/nightly/release.yaml
```

It will apply the release.yaml to your kubernetes cluster, creating the
admin namespace `pipelines-as-code`, the roles and all other bits needed.

The `pipelines-as-code` namespace is where all the admin pipelinerun are run,
they are supposed to be accesible only by the admin.

You will need then to have events from github or others coming through to your
EventListenner so follow the next steps on how to do that.

### Github configuration

To setup Pipelines as Code on Github, you need to have a Github App created.

You need the Webhook of the app pointing to your Ingress endpoint which would
then go to the triggers enventlistenner/service.

You need to make sure you have those permissions and events checked on the
GitHub app :

```json
             "default_permissions": {
                 "checks": "write",
                 "contents": "write",
                 "issues": "write",
                 "members": "read",
                 "metadata": "read",
                 "organization_plan": "read",
                 "pull_requests": "write"
             },
             "default_events": [
                 "commit_comment",
                 "issue_comment",
                 "pull_request",
                 "pull_request_review",
                 "pull_request_review_comment",
                 "push"
             ]
```

The screenshot on how it looks like is locate [here](https://user-images.githubusercontent.com/98980/124132813-7e53f580-da81-11eb-9eb4-e4f1487cf7a0.png)



When you have created the `github-app-secret` Secret, grab the private key the
`application_id` and the `webhook_secret`  from the interface, place the private
key in a file named for example `/tmp/github.app.key` and issue those commands :

```bash
% kubectl -n pipelines-as-code create secret generic github-app-secret \
        --from-literal private.key="$(cat /tmp/github.app.key)"
        --from-literal application_id="APPLICATION_ID_NUMBER" \
        --from-literal webhook.secret="WEBHOOK_SECRET"
```

This secret is used to generate a token on behalf of the user running the event
and make sure to validate the webhook via the webhook secret.

You will then need to make sure to expose the `EventListenner` via a
[Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) or a
[OpenShift
Route](https://docs.openshift.com/container-platform/latest/networking/routes/route-configuration.html)
so GitHub can get send the webhook to it.

### GitHub Enteprise

Pipelines as Code supports Github Enterprise.

You don't need to do anything special to get Pipelines as code working with GHE.
Pipelines as code will automatically detects the header as set from GHE and use it  the GHE API auth url instead of the public github.

## Configuration

There is a few things you can configure via the configmap `pipelines-as-code` in
the `pipelines-as-code` namespace.

- **application-name**: The name of the application showing for example in the
  GitHub Checks labels. Default to `"Pipelines as Code"`
- **max-keep-days**: The number of the day to keep the PR runs in the
  `pipelines-as-code` namespace, see below for more details about it..

### PR cleanups in pipelines-as-code admin namespace

We install by default a cron that cleanups the PR generated on events in pipelines-as-code
namespace. The crons runs every hour and by default cleanups pipelineruns over a
day. If you would like to change the max number of days to keep you can change the
key `max-keep-days` in the `pipelines-as-code` configmap. This configmap
setting doens't affect the cleanups of the user's PR controlled by the
annotations.

## CLI

OpenShift Pipelines CLI offer a easy to use CLI to manage your repositories status.

## Binary releases

You can grab the latest binary directly from the
[releases](https://github.com/openshift-pipelines/pipelines-as-code/releases)
page.

## Dev release

If you want to install from the git repository you can just do :

```shell
go install github.com/openshift-pipelines/pipelines-as-code/cmd/tkn-pac
```

## Brew release

On LinuxBrew or OSX brew you can simply add the tap :

```shell
brew install openshift-pipelines/pipelines-as-code/tektoncd-pac
```
