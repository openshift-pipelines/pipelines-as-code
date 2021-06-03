SETUP
=====

## Dev setup

* Configure and use [ko](https://github.com/google/ko) to easily push your change to a
  registry.
* You should be able to use it against a kind/minikube cluster but bear in mind
  we are trying to be compatible from at least the pipeline/triggers version
  from openshift-pipelines.
* Install your cluster with ko or from release yaml :

    ```shell
    make releaseyaml|kubectl apply -f-
    ```

* Configure a GitHub APP and a `github-secret` as mentionned in README.

* Configure a few Repository CR and a namespace target.

* If you go to your GitHub app setting in `Advanced` you can see the json and
  payload GitHub is sending to the eventlistenner.

* If you want to replay an event without having to `git commmit --amend
  --no-edit && git push --force`, you can capture that json blob into a file
  (ie: /tmp/payload.json) and do :

  ```shell
  ./hack/dev/replay-gh-events.py /tmp/payload.json
  ```

  That script would detect github webhook secret and payload content and replay
  it to the event listenner.

  If you don't have a OpenShift route setup to the eventlistenner, you can
  override the route with :

  ```shell
  export EL_ROUTE=http://localhost:8080
  ```

  This in combination with an always running port-forward to the eventlistenner :

  ```shell
  kubectl port-forward -n pipelines-as-code deployment/el-pipelines-as-code-interceptor 8080 8888
  ```

  will give you an easy way to debug payloads on kind.

* If you need to debug your binary from your IDE, you will need an user token
  generated from the app, at first you need to grab the installation_id of your
  repository id, you can get that from the payload :

  ```shell
  jq .installation_id /tmp/payload.json
  ```

  Then generate a token for that installation_id using the `gen-token.py`
  script, it will use your private.key and application_id from the
  `github-app-secret` secret :

  ```shell
  ./hack/dev/gen-token.py --installation-id 123456789 --cache-file /tmp/token.for.my.repo
  ```

  This only can be used for that installation_id if you target another repo it will fail.

  You want to use a cache-file or it will generate a token every time and you
  may get ratelimited.

  You can then run the binary with :

  ```shell
  go run cmd/pipelines-as-code/main.go --trigger-target=issue-recheck --webhook-type=check_run --payload-file=/tmp/payload.json --token=$(cat /tmp/token.for.my.repo)
  ```

  To guess the trigger target and webhook-type you can simply get the logs of
  the pipelines-as-code pr pod which gets printed there (along with a token you
  can use quickly).

  You can plug that command into your IDE of choice for debugging.

  On vscode argument cannot be a shell script (i.e: that cat command would not
  work), you can use [this
  plugin](https://marketplace.visualstudio.com/items?itemName=augustocdias.tasks-shell-input)
  for that.

* There is a bunch of payload example in test/fixtures/ which you can use for
  introspection. [`jid`](https://github.com/simeji/jid) tool can help.

* What I (chmouel) do is to have a test repository ([chmouel/scratchpad](https://github.com/chmouel/scratchpad)) with
  a [PR](https://github.com/chmouel/scratchmyback/pull/1) with different tasks in my tekton dir.

  I grab the SHA locally and generate a payload out of it :

  My shell script has a few variables which I configure and it autodetect from
  the local repository the REF and SHA :

  ```shell
  OWNER_REPO=${1:-"chmouel/scratchmyback"}
  INSTALLATION_ID=${2:-1234567}

  OWNER=${OWNER_REPO%/*}
  REPOSITORY=${OWNER_REPO#*/}
  REPOSITORY_URL=https://github.com/${OWNER}/${REPOSITORY}
  SENDER=${WEBHOOK_SENDER:-${USER}}
  WEBHOOK_TYPE=pull_request
  TRIGGER_TARGET=pull_request
  DEFAULT_BRANCH=main
  HEAD_BRANCH=${pushd $GOPATH/src/github.com/${OWNER)${REPO} >/dev/null && git rev-parse --abbrev-ref HEAD && popd >/dev/null}
  SHA=$(pushd $GOPATH/src/github.com/${OWNER)${REPO} >/dev/null && git rev-parse ${HEAD_BRANCH} && popd >/dev/null)
  TOKEN=$(./hack/dev/gen-token.py --installation-id ${INSTALLATION_ID} --cache-file /tmp/ghtoken.${OWNER}-${REPOSITORY}
  ```

  then generate a payload file out of it :
  ```json
   {
    "repository": {
        "owner": {
            "login": "${OWNER}"
        },
        "name": "${REPOSITORY}",
        "default_branch": "${DEFAULT_BRANCH}",
        "html_url": "${REPOSITORY_URL}"
    },
    "pull_request": {
        "user": {
            "login": "${SENDER}"
        },
        "base": {
            "ref": "${DEFAULT_BRANCH}"
        },
        "head": {
            "sha": "${SHA}",
            "ref": "${HEAD_BRANCH}"
        }
    }
    }
    ```

  which you can the pass to a go run :

  ```shell
  go run cmd/pipelines-as-code/main.go run --token="${TOKEN}" --payload-file="/tmp/payload.json"  \
   --webhook-type="${WEBHOOK_TYPE}" --trigger-target="${TRIGGER_TARGET}"
  ```

  I let the full script as an exercise to the reader.

## Code

* 100% coverage is not a goal, coverage of corner case errors handling
  is really not necessary.
* Make sure you make it easy to reproduce the bug for reviewers, i.e: copy the problematic payload in the `test/fixtures`
  directory.
