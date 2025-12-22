#!/usr/bin/env -S uv --quiet run --script
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "requests",
# ]
# ///
# pylint: disable=consider-using-f-string
#
# Provision Forgejo instance with a username password and Repository for Pipelines as Code
import os
import pathlib
import subprocess
import sys
import tempfile
import time

import requests

FORGEJO_USER = os.environ.get("FORGEJO_USER", "pac")
FORGEJO_PASSWORD = os.environ.get("FORGEJO_PASSWORD", "pac")
FORGEJO_HOST = os.environ.get("FORGEJO_HOST", "forgejo.paac-127-0-0-1.nip.io")
FORGEJO_URL = os.environ.get("FORGEJO_URL", f"http://{FORGEJO_HOST}")
FORGEJO_NS = os.environ.get("FORGEJO_NS", "forgejo")
FORGEJO_REPO_NAME_E2E = os.environ.get("FORGEJO_REPO_NAME", "pac-e2e")
FORGEJO_REPO_NAME_PERSO = os.environ.get("FORGEJO_REPO_NAME_PERSO", "pac")
OPENSHIFT_ROUTE_FORCE_HTTP = os.environ.get("OPENSHIFT_ROUTE_FORCE_HTTP", False)
PAC_CONTROLLER_NAMESPACE = os.environ.get(
    "PAC_CONTROLLER_NAMESPACE", "pipelines-as-code"
)

FORGEJO_SMEE_HOOK_URL = os.environ.get("TEST_GITEA_SMEEURL", "")  # will fail if not set
if FORGEJO_SMEE_HOOK_URL == "":
    print(
        "You need to setup a Hook URL in https://hook.pipelinesascode.com and set it up as environment variable in the `TEST_GITEA_SMEEURL` variable"
    )
    sys.exit(1)

FORGEJO_REPOS = {
    # Add some repo to provision if you like here
    # "FORGEJO_REPO_NAME_E2E": {"name": FORGEJO_REPO_NAME_E2E, "create_crd": False},
    # "FORGEJO_REPO_NAME": {"name": FORGEJO_REPO_NAME_PERSO, "create_crd": True},
}


class ForgejoDeployException(Exception):
    pass


class ProvisionForgejo:
    forgejo_host = FORGEJO_HOST
    forgejo_url = FORGEJO_URL
    namespace = PAC_CONTROLLER_NAMESPACE
    headers = {"Content-Type": "application/json"}
    token_name = "token"

    def apply_deployment_template(self):
        tmpl = os.path.join(os.path.dirname(__file__), "forgejo-deployment.yaml")
        with open(tmpl, encoding="utf-8") as fp:
            replaced = (
                fp.read()
                .replace("EMPTYBRACKET", "{}")
                .replace("VAR_FORGEJO_HOST", self.forgejo_host)
                .replace("VAR_FORGEJO_URL", self.forgejo_url)
                .replace("VAR_FORGEJO_SMEE_HOOK_URL", FORGEJO_SMEE_HOOK_URL)
                .replace(
                    "VAR_URL",
                    f"http://pipelines-as-code-controller.{self.namespace}:8080",
                )
            )
            self.apply_kubectl(replaced)
            fp.close()

    def wait_for_forgejo_to_be_up(self) -> bool:
        i = 0
        print(
            f"Waiting for Forgejo to be up on {self.forgejo_url}",
        )
        while i != 120:
            try:
                r = requests.get(
                    f"{self.forgejo_url}/api/v1/version",
                    verify=not OPENSHIFT_ROUTE_FORCE_HTTP,
                    timeout=300,
                )
                if r.status_code == 200:
                    # wait a bit more that it finishes
                    time.sleep(5)
                    print(f"installed Forgejo version is: {r.json()['version']}")
                    return True
                r.raise_for_status()
            except (requests.exceptions.ConnectionError, requests.exceptions.HTTPError):
                # Forgejo may not be up yet; ignore transient errors and retry until timeout
                pass
            i = i + 1
            time.sleep(1)
        print("failed.")
        return False

    @classmethod
    def create_user_in_pod(cls):
        # Forgejo uses the same gitea binary name for CLI commands
        gitea_cmd = (
            "/usr/local/bin/gitea -c /data/gitea/conf/app.ini admin user list | "
            f"grep -qw {FORGEJO_USER} || "
            "/usr/local/bin/gitea -c /data/gitea/conf/app.ini admin user create "
            f"--username {FORGEJO_USER} --password {FORGEJO_PASSWORD} "
            "--admin --access-token --email pac@pac.com"
        )
        try:
            subprocess.run(
                [
                    "kubectl",
                    "-n",
                    FORGEJO_NS,
                    "exec",
                    "deploy/forgejo",
                    "--",
                    "/bin/sh",
                    "-c",
                    gitea_cmd,
                ],
                check=True,
                stdout=subprocess.DEVNULL,
            )
        except subprocess.CalledProcessError:
            print("cannot run gitea admin command in pod")
            sys.exit(1)

    def create_user_in_forgejo(self):
        data_user = {
            "user_name": FORGEJO_USER,
            "password": FORGEJO_PASSWORD,
            "retype": FORGEJO_PASSWORD,
            "email": "pac@pac.com",
        }
        resp = requests.post(
            url=f"{self.forgejo_url}/user/sign_up",
            data=data_user,
            headers=self.headers,
            verify=not OPENSHIFT_ROUTE_FORCE_HTTP,
            timeout=300,
        )
        resp.raise_for_status()

    def create_repo(self, reponame: str):
        repo_data = {"auto_init": True, "name": reponame}
        resp = requests.post(
            url=f"{self.forgejo_url}/api/v1/user/repos",
            headers=self.headers,
            timeout=300,
            auth=(FORGEJO_USER, FORGEJO_PASSWORD),
            verify=not OPENSHIFT_ROUTE_FORCE_HTTP,
            json=repo_data,
        )
        resp.raise_for_status()

    def create_repo_hook(self, reponame: str):
        # Forgejo supports gitea webhook type for compatibility
        hook_data = {
            "type": "gitea",
            "config": {"url": FORGEJO_SMEE_HOOK_URL, "content_type": "json"},
            "events": ["push", "pull_request", "issue_comments"],
            "active": True,
        }
        resp = requests.post(
            url=f"{self.forgejo_url}/api/v1/repos/{FORGEJO_USER}/{reponame}/hooks",
            headers=self.headers,
            verify=not OPENSHIFT_ROUTE_FORCE_HTTP,
            auth=(FORGEJO_USER, FORGEJO_PASSWORD),
            timeout=300,
            json=hook_data,
        )
        resp.raise_for_status()

    def create_token_for_user(self) -> str:
        requests.delete(
            url=f"{self.forgejo_url}/api/v1/users/{FORGEJO_USER}/tokens/{self.token_name}",
            headers=self.headers,
            verify=not OPENSHIFT_ROUTE_FORCE_HTTP,
            timeout=300,
            auth=(FORGEJO_USER, FORGEJO_PASSWORD),
        )
        # Forgejo 13+ requires scopes for token creation
        token_data = {
            "name": self.token_name,
            "scopes": [
                "write:repository",
                "write:user",
                "write:issue",
                "write:organization",
                "write:notification",
                "write:misc",
            ],
        }
        resp = requests.post(
            url=f"{self.forgejo_url}/api/v1/users/{FORGEJO_USER}/tokens",
            headers=self.headers,
            auth=(FORGEJO_USER, FORGEJO_PASSWORD),
            verify=not OPENSHIFT_ROUTE_FORCE_HTTP,
            timeout=300,
            json=token_data,
        )
        resp.raise_for_status()
        token = resp.json()["sha1"]
        return token

    def create_repo_crd(self, repo_name, token: str):
        template = f"""
---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: forgejo-{repo_name}
spec:
  url: "{self.forgejo_url}/{FORGEJO_USER}/{repo_name}"
  git_provider:
    user: "git"
    url: "{FORGEJO_URL}"
    secret:
      name: "forgejo-localhost"
      key: token
    webhook_secret:
      name: "forgejo-localhost"
      key: "webhook"

---
apiVersion: v1
kind: Secret
metadata:
  name: forgejo-localhost
type: Opaque
stringData:
  token: "{token}"
  webhook: ""
        """
        self.apply_kubectl(template)

    @classmethod
    def apply_kubectl(cls, template: str, ns: str = ""):
        # write string to a temporary file
        args = f"-n {ns}" if ns else f"-n {FORGEJO_NS}"

        tmpfile = pathlib.Path(tempfile.mktemp("secretpaaaaccc"))
        tmpfile.write_text(template, encoding="utf-8")
        subprocess.run(
            ["kubectl", "apply"] + args.split() + ["-f", str(tmpfile)], check=True
        )
        tmpfile.unlink()

    @classmethod
    def create_ns(cls):
        subprocess.run(
            f'/bin/sh -c "kubectl get ns -o name {FORGEJO_NS} >/dev/null || kubectl create ns {FORGEJO_NS}"',
            shell=True,
            check=True,
        )

    def create_ingress_or_route(self):
        # detect if we are running on openshift
        openshift = True
        try:
            subprocess.run(
                '/bin/sh -c "kubectl get routes.route.openshift.io"',
                shell=True,
                check=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        except subprocess.CalledProcessError:
            openshift = False

        if not openshift:
            # Apply ingress for kind/kubernetes clusters
            ingress_file = os.path.join(
                os.path.dirname(__file__), "ingress-forgejo.yaml"
            )
            subprocess.run(
                ["kubectl", "apply", "-f", ingress_file],
                check=True,
            )
            return

        if openshift:
            tls_mode = ""
            if not OPENSHIFT_ROUTE_FORCE_HTTP:
                tls_mode = """
  tls:
    insecureEdgeTerminationPolicy: Redirect
    termination: edge
            """
            template = f"""---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: forgejo
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: pipelines-as-code
    app.kubernetes.io/version: "devel"
    pipelines-as-code/route: controller
spec:
  port:
    targetPort: http-listener
  {tls_mode}
  to:
    kind: Service
    name: forgejo
    weight: 100
  wildcardPolicy: None
apiVersion: route.openshift.io/v1
"""
            self.apply_kubectl(template)
            time.sleep(5)
            self.forgejo_host = subprocess.run(
                f"/bin/sh -c \"kubectl get routes.route.openshift.io -n {FORGEJO_NS} -o jsonpath='{{.items[0].spec.host}}'\"",
                shell=True,
                check=True,
                capture_output=True,
                text=True,
            ).stdout
            prefix = "http" if OPENSHIFT_ROUTE_FORCE_HTTP else "https"
            self.forgejo_url = f"{prefix}://{self.forgejo_host}"


def main():
    m = ProvisionForgejo()
    m.create_ns()
    m.create_ingress_or_route()
    m.apply_deployment_template()
    if not m.wait_for_forgejo_to_be_up():
        raise ForgejoDeployException(f"Could not get Forgejo on {m.forgejo_url}")
    m.create_user_in_pod()
    m.create_user_in_forgejo()
    token = m.create_token_for_user()
    for _, config in FORGEJO_REPOS.items():
        m.create_repo(config["name"])
        m.create_repo_hook(config["name"])
        if config["create_crd"]:
            m.create_repo_crd(config["name"], token)
    print(
        f"SUCCESS: Forgejo is available on {m.forgejo_url}\n"
        f"User: {FORGEJO_USER} Password: {FORGEJO_PASSWORD} Token: {token}"
    )


if "__main__" == __name__:
    main()
