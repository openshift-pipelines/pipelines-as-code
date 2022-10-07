#!/usr/bin/env python3
# pylint: disable=no-self-use, disable=consider-using-f-string
#
# Provision gitea instance with a username password and Repository for Pipelines as Code

import os
import time
import subprocess
import tempfile
import sys

import requests

GITEA_IS_HTTPS = os.environ.get("GITEA_IS_HTTPS", "true")
GITEA_USER = os.environ.get("GITEA_USER", "pac")
GITEA_PASSWORD = os.environ.get("GITEA_PASSWORD", "pac")
GITEA_HOST = os.environ.get("GITEA_HOST", "localhost:3000")
GITEA_URL = os.environ.get("GITEA_URL", f"http://{GITEA_HOST}")
GITEA_NS = os.environ.get("GITEA_NS", "gitea")
GITEA_REPO_NAME_E2E = os.environ.get("GITEA_REPO_NAME", "pac-e2e")
GITEA_REPO_NAME_PERSO = os.environ.get("GITEA_REPO_NAME_PERSO", "pac")

GITEA_SMEE_HOOK_URL = os.environ.get("TEST_GITEA_SMEEURL", "")  # will fail if not set
if GITEA_SMEE_HOOK_URL == "":
    print(
        "You need to setupo a SMEE URL in https://smee.io and set it up as environement variable in the `TEST_GITEA_SMEEURL` variable"
    )
    sys.exit(1)

GITEA_REPOS = {
    # Add some repo to provision if you like here
    # "GITEA_REPO_NAME_E2E": {"name": GITEA_REPO_NAME_E2E, "create_crd": False},
    # "GITEA_REPO_NAME": {"name": GITEA_REPO_NAME_PERSO, "create_crd": True},
}


class ProvisionGitea:
    gitea_host = GITEA_HOST
    gitea_url = GITEA_URL
    headers = {"Content-Type": "application/json"}
    token_name = "token"

    def apply_deployment_template(self):
        tmpl = os.path.join(os.path.dirname(__file__), "gitea-deployment.yaml")
        fp = open(tmpl)
        replaced = (
            fp.read()
            .replace("EMPTYBRACKET", "{}")
            .replace("VAR_GITEA_HOST", self.gitea_host)
            .replace("VAR_GITEA_URL", self.gitea_url)
            .replace("VAR_GITEA_SMEE_HOOK_URL", GITEA_SMEE_HOOK_URL)
        )
        self.apply_kubectl(replaced)
        fp.close()

    def wait_for_gitea_to_be_up(self) -> bool:
        i = 0
        print(
            f"Waiting for gitea to be up on {self.gitea_url}",
        )
        while i != 120:
            try:
                r = requests.get(f"{self.gitea_url}/api/v1/version", verify=False)
                if r.status_code == 200:
                    # wait a bit more that it finishes
                    time.sleep(5)
                    return True
                r.raise_for_status()
            except (requests.exceptions.ConnectionError, requests.exceptions.HTTPError):
                pass
            i = i + 1
            time.sleep(1)
        print("failed.")
        return False

    def create_user_in_pod(self):
        subprocess.run(
            f"/bin/sh -c \"kubectl -n {GITEA_NS} exec $(kubectl -n {GITEA_NS} get pod --field-selector=status.phase==Running  -l app=gitea -o name|sed 's,.*/,,') --  /bin/bash -c './gitea -c /home/gitea/conf/app.ini admin  user  list|grep -w pac || ./gitea -c /home/gitea/conf/app.ini admin user create --username pac --password pac --admin --access-token --email pac@pac.com'\"",
            shell=True,
            check=True,
            stdout=subprocess.DEVNULL,
        )

    def create_user_in_gitea(self):
        data_user = {
            "user_name": GITEA_USER,
            "password": GITEA_PASSWORD,
            "retype": GITEA_PASSWORD,
            "email": "pac@pac.com",
        }
        resp = requests.post(
            url=f"{self.gitea_url}/user/sign_up",
            data=data_user,
            headers=self.headers,
            verify=False,
        )
        resp.raise_for_status()

    def create_repo(self, reponame: str):
        jeez = """ {"auto_init": true, "name": "%s"} """ % (reponame)
        resp = requests.post(
            url=f"{self.gitea_url}/api/v1/user/repos",
            headers=self.headers,
            auth=(GITEA_USER, GITEA_PASSWORD),
            data=jeez,
            verify=False,
        )
        resp.raise_for_status()

    def create_repo_hook(self, reponame: str):
        jeez = (
            """{"type": "gitea", "config": { "url": "%s", "content_type": "json"}, "events": ["push", "pull_request", "issue_comments"], "active": true}"""
            % (GITEA_SMEE_HOOK_URL)
        )
        resp = requests.post(
            url=f"{self.gitea_url}/api/v1/repos/{GITEA_USER}/{reponame}/hooks",
            headers=self.headers,
            auth=(GITEA_USER, GITEA_PASSWORD),
            data=jeez,
            verify=False,
        )
        resp.raise_for_status()

    def create_token_for_user(self) -> str:
        requests.delete(
            url=f"{self.gitea_url}/api/v1/users/{GITEA_USER}/tokens/{self.token_name}",
            headers=self.headers,
            auth=(GITEA_USER, GITEA_PASSWORD),
            verify=False,
        )
        jeez = """{"name": "%s"}""" % (self.token_name)
        resp = requests.post(
            url=f"{self.gitea_url}/api/v1/users/{GITEA_USER}/tokens",
            headers=self.headers,
            auth=(GITEA_USER, GITEA_PASSWORD),
            data=jeez,
            verify=False,
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
  name: gitea-{repo_name}
spec:
  url: "{self.gitea_url}/{GITEA_USER}/{repo_name}"
  git_provider:
    user: "git"
    url: "{GITEA_URL}"
    secret:
      name: "gitea-localhost"
      key: token
    webhook_secret:
      name: "gitea-localhost"
      key: "webhook"

---
apiVersion: v1
kind: Secret
metadata:
  name: gitea-localhost
type: Opaque
stringData:
  token: "{token}"
  webhook: ""
        """
        self.apply_kubectl(template)

    def apply_kubectl(self, template: str, ns: str = ""):
        # write string to a temporary file
        args = f"-n {ns}" if ns else f"-n {GITEA_NS}"

        tmp = tempfile.mktemp("secretpaaaaccc")
        open(tmp, "w", encoding="utf-8").write(template)
        os.system(f"kubectl apply {args} -f {tmp}")
        os.remove(tmp)

    def create_ns(self):
        subprocess.run(
            f'/bin/sh -c "kubectl get ns -o name {GITEA_NS} >/dev/null || kubectl create ns {GITEA_NS}"',
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

        if openshift:
            template = """---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: gitea
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: pipelines-as-code
    app.kubernetes.io/version: "devel"
    pipelines-as-code/route: controller
spec:
  port:
    targetPort: http-listener
  tls:
    insecureEdgeTerminationPolicy: Redirect
    termination: edge
  to:
    kind: Service
    name: gitea
    weight: 100
  wildcardPolicy: None
apiVersion: route.openshift.io/v1
"""
            time.sleep(2)
            self.apply_kubectl(template)
        if openshift:
            time.sleep(2)
            self.gitea_host = subprocess.run(
                f"/bin/sh -c \"kubectl get routes.route.openshift.io -n {GITEA_NS} -o jsonpath='{{.items[0].spec.host}}'\"",
                shell=True,
                check=True,
                capture_output=True,
                text=True,
            ).stdout
            self.gitea_url = f"https://{self.gitea_host}"

    def create_ingress_or_route_for_http(self):
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

        if openshift:
            template = """---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: gitea
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: pipelines-as-code
    app.kubernetes.io/version: "devel"
    pipelines-as-code/route: controller
spec:
  port:
    targetPort: http-listener
  tls:
    insecureEdgeTerminationPolicy: Redirect
    termination: edge
  to:
    kind: Service
    name: gitea
    weight: 100
  wildcardPolicy: None
apiVersion: route.openshift.io/v1
"""
            time.sleep(2)
            self.apply_kubectl(template)
        if openshift:
            time.sleep(2)
            self.gitea_host = subprocess.run(
                f"/bin/sh -c \"kubectl get routes.route.openshift.io -n {GITEA_NS} -o jsonpath='{{.items[0].spec.host}}'\"",
                shell=True,
                check=True,
                capture_output=True,
                text=True,
            ).stdout
            self.gitea_url = f"https://{self.gitea_host}"

def main():
    m = ProvisionGitea()
    m.create_ns()
    if GITEA_IS_HTTPS == "false":
        m.create_ingress_or_route_for_http()
    else:
        m.create_ingress_or_route()
    m.apply_deployment_template()
    if not m.wait_for_gitea_to_be_up():
        raise Exception(f"Could not get gitea on {m.gitea_url}")
    m.create_user_in_pod()
    m.create_user_in_gitea()
    token = m.create_token_for_user()
    for _, config in GITEA_REPOS.items():
        m.create_repo(config["name"])
        m.create_repo_hook(config["name"])
        if config["create_crd"]:
            m.create_repo_crd(config["name"], token)
    print(
        f"SUCCESS: gitea is available on {m.gitea_url}\n"
        f"User: {GITEA_USER} Password: {GITEA_PASSWORD} Token: {token}"
    )


if "__main__" == __name__:
    main()
