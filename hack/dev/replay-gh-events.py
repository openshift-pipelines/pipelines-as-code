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
"""Will replay a json file in Eventlistenner, it automatically detects the
EventListenner, the webhook secret from pipelines-as-code-secret secret name
require requests library"""
import argparse
import base64
import hashlib
import hmac
import json
import subprocess
import os

import requests

NAMESPACE = "pipelines-as-code"
SECRET_NAME = "pipelines-as-code-secret"
ELNAME = "pipelines-as-code"


def get_el_route():
    elroute = subprocess.run(
        f"oc get route -n {NAMESPACE} -l eventlistener={ELNAME}-interceptor -o json",
        shell=True,
        check=True,
        capture_output=True)
    return "https://" + \
        json.loads(elroute.stdout)["items"][0]["status"]["ingress"][0]["host"]


def get_installation_id_and_webhook_secret():
    secret = subprocess.run(
        f"kubectl get secret {SECRET_NAME} -n{NAMESPACE} -o json",
        shell=True,
        check=True,
        capture_output=True)
    jeez = json.loads(secret.stdout)
    return (jeez["data"]["github-application-id"],
            base64.b64decode(jeez["data"]["webhook.secret"]).decode())


def main(args):
    application_id, secret = get_installation_id_and_webhook_secret()
    el = args.eroute and args.eroute or get_el_route()
    text = open(args.json_file).read()
    jeez = json.loads(text)
    esha256 = hmac.new(secret.encode("utf-8"),
                       msg=text.encode("utf-8"),
                       digestmod=hashlib.sha256).hexdigest()
    esha1 = hmac.new(secret.encode("utf-8"),
                     msg=text.encode("utf-8"),
                     digestmod=hashlib.sha1).hexdigest()

    print("Replay event for repo " + jeez["repository"]["full_name"])
    if 'action' in jeez and jeez["action"] in (
            "opened", "synchronize") and "pull_request" in jeez:
        event_type = "pull_request"
    elif 'action' in jeez and jeez[
            "action"] == "rerequested" and "check_run" in jeez:
        event_type = "check_run"
    elif 'action' in jeez and jeez["action"] == "created" and "issue" in jeez:
        event_type = "issue_comment"
    elif 'pusher' in jeez:
        event_type = "push"
    else:
        raise Exception("Unknown event_type")

    headers = {
        "content-type": "application/json",
        "X-GitHub-Event": event_type,
        "X-GitHub-Hook-Installation-Target-ID": application_id,
        "X-GitHub-Hook-Installation-Target-Type": "integration",
        "X-Hub-Signature": "sha1=" + esha1,
        "X-Hub-Signature-256": "sha256=" + esha256,
    }
    r = requests.request("POST",
                         el,
                         data=text.encode("utf-8"),
                         headers=headers)
    print(r.content.decode())


def parse_args():
    parser = argparse.ArgumentParser(description='Replay a webhook')
    parser.add_argument("--eventlistenner-route",
                        dest="eroute",
                        help="Route hostname (default to detect on openshift)",
                        default=os.environ.get("EL_ROUTE"))
    parser.add_argument("json_file", help="json file name")
    return parser.parse_args()


if __name__ == '__main__':
    main(parse_args())
