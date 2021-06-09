#!/usr/bin/env python3
# Upload a file to github directly to a branch
# i.e: upload-file-to-github.py --branch-ref refs/heads/nightly \
#    --owner-repository openshift-pipelines/pipelines-as-code \
#    --token $(git config --get github.oauth-token) \
#    --message "Automatically uploaded from branch blah" \
#    --destination release.yaml --filename <(./hack/generate-releaseyaml.sh)
import argparse
import base64
import http.client
import json
import urllib


def github_request(token: str,
                   method: str,
                   url: str,
                   headers=None,
                   data=None,
                   params=None):
    if not headers:
        headers = {}

    headers.setdefault("Authorization", "Bearer " + token)
    headers.setdefault("User-Agent", "Tekton asa Code")

    if not url.startswith("https://"):
        url = "https://api.github.com" + url

    url_parsed = urllib.parse.urlparse(url)
    url_path = url_parsed.path
    if params:
        url_path += "?" + urllib.parse.urlencode(params)

    data = data and json.dumps(data)
    conn = http.client.HTTPSConnection(url_parsed.hostname)
    conn.request(method, url_path, body=data, headers=headers)
    response = conn.getresponse()
    if response.status == 302:
        return github_request(token, method, response.headers["Location"])

    if response.status >= 400:
        headers.pop('Authorization', None)
        raise Exception(
            f"Error: {response.status} - {json.loads(response.read())} - {method} - {url} - {data} - {headers}"
        )

    return (response, json.loads(response.read()))


def upload_to_github(token, repository, src, dst, branch, msg):
    # Get last commit SHA of a branch
    resp, jeez = github_request(token, "GET",
                                f"/repos/{repository}/git/{branch}")
    last_commit_sha = jeez["object"]["sha"]
    print("Last commit SHA: " + last_commit_sha)

    base64content = base64.b64encode(open(src, "rb").read())
    resp, jeez = github_request(
        token,
        "POST",
        f"/repos/{repository}/git/blobs",
        data={
            "content": base64content.decode(),
            "encoding": "base64"
        },
    )
    blob_content_sha = jeez["sha"]

    resp, jeez = github_request(
        token,
        "POST",
        f"/repos/{repository}/git/trees",
        data={
            "base_tree":
            last_commit_sha,
            "tree": [{
                "path": dst,
                "mode": "100644",
                "type": "blob",
                "sha": blob_content_sha,
            }],
        },
    )
    tree_sha = jeez["sha"]

    resp, jeez = github_request(
        token,
        "POST",
        f"/repos/{repository}/git/commits",
        data={
            "message": msg,
            "author": {
                "name": "Tekton as a Code",
                "email": "pipelines@redhat.com",
            },
            "parents": [last_commit_sha],
            "tree": tree_sha,
        },
    )
    new_commit_sha = jeez["sha"]

    resp, jeez = github_request(
        token,
        "PATCH",
        f"/repos/{repository}/git/{branch}",
        data={"sha": new_commit_sha},
    )
    return (resp, jeez)


def parse_args():
    parser = argparse.ArgumentParser(description='Upload a file to github ref')
    parser.add_argument("--filename", "-f", required=True)
    parser.add_argument("--message", "-m", required=True)
    parser.add_argument("--destination", "-d", required=True)
    parser.add_argument("--branch-ref", "-r", required=True)
    parser.add_argument("--owner-repository", "-o", required=True)
    parser.add_argument("--token", "-t", required=True)
    return parser.parse_args()


def main(args):
    resp, jz = upload_to_github(args.token, args.owner_repository,
                                args.filename, args.destination,
                                args.branch_ref, args.message)
    print(resp.status)


if __name__ == "__main__":
    main(parse_args())
