# GitLab

## Supported

- ACL (Access Control List) on project or group
- `/ok-to-test` support from allowed users
- `/retest` support.
- `/retest` on a non owner mr from a owner will retest it but subsequent submissions
  would not be allowed. (only ok-to-test allows it)
- OWNERS files
- Report build status on MR if token has access of submitted source
- Report pull request status via comments if no access
- Get files via APIs
- Webhook via api token attached to repo secret
- Auto secret attached for git-clone support on private repo.
- Private instance is supported, the private instance is not required to be
  specified in config since auto detect but if for whatever reason you want to
  set another api url you can do this on Repository :

  ```yaml
  kind: Repository
  spec:
    url: https://apiurl/project/repo
    provider:
      url: https://apiurl
      secret:
        name: secret-ref
        key: token
  ```

## Fork Limitations and Status Reporting

When a Merge Request originates from a forked repository to an upstream repository,
Pipelines-as-Code uses a three-layer fallback mechanism for status reporting:

1. **Source Project (Fork)**: First attempt to set commit status
   - Requires: Token with write access to fork repository
   - Common failure: Token lacks fork permissions

2. **Target Project (Upstream)**: Fallback to setting status on upstream
   - Requires: Token with CI pipeline access to upstream
   - May fail: GitLab only creates pipeline entries when CI runs in that project

3. **Merge Request Comments**: Final fallback to posting status as comment
   - Always succeeds (requires MR write permissions)
   - Provides same information as API status checks

**Comment Strategy Control:**

Disable status comments (will still attempt API status updates):

```yaml
spec:
  settings:
    gitlab:
      comment_strategy: "disable_all"
```

**For detailed troubleshooting and workarounds**, see:
[GitLab Installation - Troubleshooting Fork Merge Requests](../../../docs/content/docs/install/gitlab.md#troubleshooting-fork-merge-requests)

**For technical implementation details**, see:
[Repository CRD - GitLab comment strategy](../../../docs/content/docs/guide/repositorycrd.md#gitlab)

## TODO

### Until there is a ask for it

- caching API calls for permissions.

## NOTES

- since orgs may have subpaths we switch the / to - so we can use it for the pac
  automatic secret and label. Sucks a bit but such is life
