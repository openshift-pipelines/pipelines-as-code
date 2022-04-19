#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Author: Chmouel Boudjnah <chmouel@chmouel.com>
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.
# See README.md for documentation
import typing
import argparse
import base64
import hashlib
import hmac
import json
import os
import subprocess
import sys
import time

import requests
import ghapp_token

NAMESPACE = "pipelines-as-code"
SECRET_NAME = "pipelines-as-code-secret"
ELNAME = "pipelines-as-code"


EXPIRE_MINUTES_AS_SECONDS = (
    int(os.environ.get("GITHUBAPP_TOKEN_EXPIRATION_MINUTES", 10)) * 60
)


def get_controller_route():
    elroute = subprocess.run(
        f"kubectl get route -n {NAMESPACE} -l pipelines-as-code/route=controller -o json",
        shell=True,
        check=True,
        capture_output=True,
    )
    return (
        "https://"
        + json.loads(elroute.stdout)["items"][0]["status"]["ingress"][0]["host"]
    )


def get_controller_ingress():
    elroute = subprocess.run(
        f"kubectl get ingress -n {NAMESPACE} -l pipelines-as-code/route=controller -o json",
        shell=True,
        check=True,
        capture_output=True,
    )
    return (
        "http://" + json.loads(elroute.stdout)["items"][0]["spec"]["rules"][0]["host"]
    )


def get_token_secret(
    github_api_url=ghapp_token.GITHUB_API_URL, expiration_time=EXPIRE_MINUTES_AS_SECONDS
):
    secret = subprocess.run(
        f"kubectl get secret {SECRET_NAME} -n{NAMESPACE} -o json",
        shell=True,
        check=True,
        capture_output=True,
    )
    jeez = json.loads(secret.stdout)
    private_key = base64.b64decode(jeez["data"]["github-private-key"])
    app_id = base64.b64decode(jeez["data"]["github-application-id"])
    webhook_secret = base64.b64decode(jeez["data"]["webhook.secret"]).decode()
    if not private_key or not app_id or not webhook_secret:
        print(
            f"private_key={private_key[1:10]} or app_id={app_id} or webhook_secret={webhook_secret} are empty"
        )
        sys.exit(1)

    gh = ghapp_token.GitHub(
        private_key,
        app_id,
        expiration_time,
        github_api_url,
    )
    return gh.token, webhook_secret, app_id


def _request_app_delivery(token, iid=None, api_url=ghapp_token.GITHUB_API_URL):
    url = f"{api_url}/app/hook/deliveries"
    if iid:
        url += f"/{iid}"
    headers = {
        "Accept": "application/vnd.github.v3+json",
        "Authorization": f"Bearer {token}",
    }
    return requests.request("GET", url, headers=headers)


def _request_webhooks_installed(
    token: str,
    owner_repo: str,
    iid: typing.Union[int, None] = None,
    api_url: str = ghapp_token.GITHUB_API_URL,
):
    url = f"{api_url}/repos/{owner_repo}/hooks"
    if iid:
        url += f"/{iid}/deliveries"
    headers = {
        "Accept": "application/vnd.github.v3+json",
        "Authorization": f"Bearer {token}",
    }
    return requests.request("GET", url, headers=headers)


def _request_webhooks_reattempt(
    token: str,
    owner_repo: str,
    iid: int,
    delivery_id: int,
    api_url: str = ghapp_token.GITHUB_API_URL,
):
    url = f"{api_url}/repos/{owner_repo}/hooks/{iid}/deliveries/{delivery_id}/attempts"
    print(url)
    headers = {
        "Accept": "application/vnd.github.v3+json",
        "Authorization": f"Bearer {token}",
    }
    return requests.request("POST", url, headers=headers)


def ask_which(token: str, api_url: str, last: bool, deliveries: dict) -> int:
    dico = []
    i = 1
    if "message" in deliveries:
        print(deliveries)
        sys.exit(0)
    for delivery in deliveries:
        print(
            f"{i}) Action={delivery['action']} Event={delivery['event']} Delivered at {delivery['delivered_at']}"
        )
        dico.append(delivery["id"])
        if i == 10:
            break
        i += 1
    chosen = input("Choose a delivery: ")
    # return _request_app_delivery(token, dico[int(chosen) - 1], api_url=api_url).json()
    return int(chosen) - 1


def webhook_get_delivery(
    token: str,
    owner_repo: str,
    last: bool = False,
    api_url: str = ghapp_token.GITHUB_API_URL,
) -> str:
    r = _request_webhooks_installed(token, api_url=api_url, owner_repo=owner_repo)
    r.raise_for_status()
    webhooks = r.json()
    if len(webhooks) == 1:
        webhook_id = int(webhooks[0]["id"])
    elif len(webhooks) > 1:
        cnt = 1
        for wh in webhooks:
            print(f"{cnt}) {wh['name']} - {wh['config']['url']} ")
            cnt += 1
        chosen = input("Choose a delivery: ")
        webhook_id = int(webhooks[int(chosen) - 1]["id"])
    else:
        print("could not find any webhook configuration on your repo {}")
        sys.exit(1)

    r = _request_webhooks_installed(
        token, api_url=api_url, owner_repo=owner_repo, iid=webhook_id
    )
    r.raise_for_status()
    deliveries = r.json()
    if not deliveries:
        print("no deliveries has been set ")
        sys.exit(1)
    if last:
        delivery_id = deliveries[0]["id"]
    else:
        chosen = ask_which(token, api_url, last, r.json())
        delivery_id = deliveries[chosen]["id"]

    r = _request_webhooks_reattempt(
        token=token,
        owner_repo=owner_repo,
        iid=webhook_id,
        api_url=api_url,
        delivery_id=delivery_id,
    )
    r.raise_for_status()
    print(f"Delivery has been replayed, you can replay directly it with: ")
    s = f"http POST {api_url}/repos/{owner_repo}/hooks/{webhook_id}/deliveries/{delivery_id}/attempts"
    s += f' Authorization:"Bearer { os.environ.get("PASS_TOKEN", "$TOKEN") }"'
    s += " Accept:application/vnd.github.v3+json"
    print(s)
    return s


def app_get_delivery(
    token: str, last: bool = False, api_url: str = ghapp_token.GITHUB_API_URL
) -> dict:
    r = _request_app_delivery(token, api_url=api_url)
    r.raise_for_status()
    deliveries = r.json()
    if not deliveries:
        print("no deliveries has been set ")
        sys.exit(1)
    if last:
        return _request_app_delivery(token, deliveries[0]["id"], api_url=api_url).json()

    chosen = ask_which(token, api_url, last, deliveries)
    return _request_app_delivery(
        token, deliveries[chosen]["id"], api_url=api_url
    ).json()


def save_script(target: str, el_route: str, headers: dict, payload: str):
    s = f"""#!/usr/bin/env python3
import requests
import sys
payload = \"\"\"{json.dumps(payload)}\"\"\"
headers={headers}
el_route = "http://localhost:8080" if (len(sys.argv) > 1 and sys.argv[1] == "-l") else "{el_route}"
r = requests.request("POST",el_route,data=payload.encode("utf-8"),headers=headers)
r.raise_for_status()
print("Request has been replayed on " + el_route)
"""
    with open(target, "w") as fp:
        fp.write(s)
    os.chmod(target, 0o755)
    print(f"Request saved to {target}")


def main(args):
    el = args.eroute
    if not el:
        try:
            el = get_controller_route()
        except subprocess.CalledProcessError:
            try:
                el = get_controller_ingress()
            except subprocess.CalledProcessError:
                print("Could not find an ingress or route")
                sys.exit(1)
    if args.webhook_repo:
        token, webhook_secret = args.webhook_token, args.webhook_secret
        replays = webhook_get_delivery(
            token,
            last=args.last_event,
            api_url=args.api_url,
            owner_repo=args.webhook_repo,
        )
        if args.save:
            open(args.save, "w").write(f"""#!/usr/bin/env bash\n{replays}\n""")
            os.chmod(args.save, 0o755)
            print(f"Saved to {args.save}")
        sys.exit(0)
    else:
        token, webhook_secret, app_id = get_token_secret(github_api_url=args.api_url)
        delivery = app_get_delivery(token, args.last_event, args.api_url)
    jeez = delivery["request"]["payload"]
    headers = delivery["request"]["headers"]
    payload = json.dumps(jeez)
    esha256 = hmac.new(
        webhook_secret.encode("utf-8"),
        msg=payload.encode("utf-8"),
        digestmod=hashlib.sha256,
    ).hexdigest()
    esha1 = hmac.new(
        webhook_secret.encode("utf-8"),
        msg=payload.encode("utf-8"),
        digestmod=hashlib.sha1,
    ).hexdigest()

    print("Replay event for repo " + jeez["repository"]["full_name"])
    headers.update(
        {
            "X-Hub-Signature": "sha1=" + esha1,
            "X-Hub-Signature-256": "sha256=" + esha256,
        }
    )

    if args.save:
        save_script(args.save, el, headers, jeez)
        sys.exit(0)
    for _ in range(args.retry):
        try:
            r = requests.request(
                "POST", el, data=payload.encode("utf-8"), headers=headers
            )
        except requests.exceptions.ConnectionError:
            print(f"sleeping until {el} is up")
            time.sleep(5)
            continue

        print(f"Payload has been replayed on {el}: {r}")
        return
    print("You have reached the maximum number of retries")


def parse_args():
    parser = argparse.ArgumentParser(description="Replay a webhook")
    parser.add_argument(
        "--installation-id",
        "-i",
        default=os.environ.get("INSTALLATION_ID"),
        help="Installation ID",
    )
    parser.add_argument(
        "--controller-route",
        "-e",
        dest="eroute",
        help="Route hostname (default to detect on openshift/ingress)",
        default=os.environ.get("EL_ROUTE"),
    )
    parser.add_argument("--last-event", "-L", action="store_true")

    parser.add_argument(
        "--webhook-repo", "-w", help="Use a webhook-repo instead of app"
    )
    parser.add_argument("--webhook-token", "-t", help="Use this token")
    parser.add_argument("--webhook-secret", "-S", help="Use this webhook secret")

    parser.add_argument(
        "--save", "-s", help="save the request to a shell script to replay easily"
    )
    parser.add_argument(
        "-a",
        "--api-url",
        help="Github API URL",
        default=os.environ.get("GITHUB_API_URL", ghapp_token.GITHUB_API_URL),
    )
    parser.add_argument(
        "--retry",
        type=int,
        default=1,
        help="how many time to try to contact the el route",
    )
    return parser.parse_args()


if __name__ == "__main__":
    main(parse_args())
