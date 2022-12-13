#!/usr/bin/env python3
import argparse
import base64
import datetime
import json
import os
import pathlib
import subprocess
import time

import requests
from jwcrypto import jwk, jwt

SECRET_NAME = "pipelines-as-code-secret"
NAMESPACE = "pipelines-as-code"
EXPIRE_MINUTES_AS_SECONDS = (
    int(os.environ.get("GITHUBAPP_TOKEN_EXPIRATION_MINUTES", 10)) * 60
)
# TODO support github enteprise
GITHUB_API_URL = "https://api.github.com"


# pylint: disable=too-few-public-methods
class GitHub:
    token = None

    def __init__(
        self, private_key, app_id, expiration_time, github_api_url, installation_id=None
    ):
        if not isinstance(private_key, bytes):
            private_key = private_key.encode()
        self._private_key = private_key
        self.app_id = app_id
        self.expiration_time = expiration_time
        self.github_api_url = github_api_url
        self.jwt_token = self._get_jwt_token()
        self.token = self._get_token(installation_id)

    @classmethod
    def _load_private_key(cls, pem_key_bytes):
        return jwk.JWK.from_pem(pem_key_bytes)

    def _get_jwt_token(self):
        key = self._load_private_key(self._private_key)
        now = int(time.time())

        token = jwt.JWT(
            header={"alg": "RS256"},
            claims={
                "iat": now,
                "exp": now + self.expiration_time,
                "iss": int(self.app_id),
            },
            algs=["RS256"],
        )
        token.make_signed_token(key)
        return token.serialize()

    def _get_token(self, installation_id):
        req = self._request(
            "POST",
            f"/app/installations/{installation_id}/access_tokens",
            headers={
                "Authorization": f"Bearer {self.jwt_token}",
                "Accept": "application/vnd.github.v3+json",
            },
        )

        if not req.text.strip():
            raise Exception(
                f"Not getting a json: code: {req.status_code} reason: {req.reason}"
            )
        ret = req.json()
        if "token" not in ret:
            raise Exception(f"Authentication errors: {req.text}")
        return ret["token"]

    def _request(self, method, url, headers=None, data=None):
        headers = headers or {}
        data = data or {}
        if self.token and "Authorization" not in headers:
            headers.update({"Authorization": "Bearer " + self.token})
        if not url.startswith("http"):
            url = f"{self.github_api_url}{url}"
        return requests.request(
            method, url, timeout=300, headers=headers, data=json.dumps(data)
        )


def get_from_pass(passkey: str):
    _application_id = subprocess.run(
        f"pass show {passkey}/github-application-id",
        shell=True,
        check=True,
        capture_output=True,
    )

    _private_key = subprocess.run(
        f"pass show {passkey}/github-private-key",
        shell=True,
        check=True,
        capture_output=True,
    )
    return (
        _application_id.stdout.decode().strip(),
        _private_key.stdout.decode().strip(),
    )


def get_private_key(ns):
    secret = subprocess.run(
        f"kubectl get secret {SECRET_NAME} -n{ns} -o json",
        shell=True,
        check=True,
        capture_output=True,
    )
    jeez = json.loads(secret.stdout)
    return (
        base64.b64decode(jeez["data"]["github-application-id"]).decode(),
        base64.b64decode(jeez["data"]["github-private-key"]),
    )


def main(args):
    if args.cache_file and os.path.exists(args.cache_file):
        mtime = os.path.getmtime(args.cache_file)
        if datetime.datetime.fromtimestamp(
            mtime
        ) < datetime.datetime.now() - datetime.timedelta(
            seconds=args.token_expiration_time
        ):
            os.remove(args.cache_file)
        else:
            print(pathlib.Path(args.cache_file).read_text(encoding="utf-8"))

    if args.pass_profile:
        application_id, private_key = get_from_pass(args.pass_profile)
    else:
        application_id, private_key = get_private_key(args.install_namespace)
    github_app = GitHub(
        private_key,
        application_id,
        expiration_time=args.token_expiration_time,
        github_api_url=args.api_url,
        installation_id=args.installation_id,
    )
    if args.jwt_token:
        print(github_app.jwt_token)
    else:
        print(github_app.token)
    if args.cache_file:
        print(
            pathlib.Path(args.cache_file).write_text(
                str(github_app.token), encoding="utf-8"
            )
        )


def parse_args():
    parser = argparse.ArgumentParser(description="Generate a user token")
    parser.add_argument(
        "--token-expiration-time",
        type=int,
        help="Token expiration time (seconds)",
        default=EXPIRE_MINUTES_AS_SECONDS,
    )
    parser.add_argument(
        "--installation-id", "-i", type=int, help="Installation_ID", required=True
    )

    parser.add_argument(
        "--pass-profile",
        "-P",
        help="Use pass to get the keys instead of detecting secret from kube, it's a folder container github-private-key and github-application-id",
    )
    parser.add_argument(
        "-n",
        "--install-namespace",
        help="Install Namespace",
        default=os.environ.get("PAC_NAMESPACE", NAMESPACE),
    )
    parser.add_argument("-j", "--jwt-token", help="Get JWT Token", action="store_true")
    parser.add_argument(
        "-a",
        "--api-url",
        help="Github API URL",
        default=os.environ.get("GITHUB_API_URL", GITHUB_API_URL),
    )
    parser.add_argument(
        "-c",
        "--cache-file",
        help=(
            f"Cache file will only regenerate after the expiration time, "
            f"default: {EXPIRE_MINUTES_AS_SECONDS / 60} minutes"
        ),
        default=os.environ.get("GITHUBAPP_RESULT_PATH"),
    )
    return parser.parse_args()


if __name__ == "__main__":
    main(parse_args())
