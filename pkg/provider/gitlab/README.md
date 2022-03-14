# Gitlab

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

## TODO

### Until there is a ask for it

- /ok-to-test in threads comments (only top level comment is supported atm).
- caching API calls for permissions.

## NOTES

- since orgs may have subpaths we switch the / to - so we can use it for the pac
  automatic secret and label. Sucks a bit but such is life
