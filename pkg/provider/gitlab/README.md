# Gitlab

## Supported

- ACL (Access Control List) on project or group
- `/ok-to-test` support from allowed users
- OWNERS files
- Report build status on MR if token has access of submitted source
- Report pull request status via comments if no access
- Get files via APIs
- Webhook via api token attached to repo secret

## TODO

- Handle paginations.
- /ok-to-test in threads comments (only top level comment is supported atm)
- caching API calls for permissions.
