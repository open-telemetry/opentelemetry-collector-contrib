# githubgen

This executable is used to generate the `.github/CODEOWNERS` and `.github/ALLOWLIST` files.

It reads status metadata from `metadata.yaml` files located throughout the repository.

It checks that codeowners are known members of the OpenTelemetry organization.

## Usage

```
$> go run cmd/githubgen/main.go --folder . [--check] [--members cmd/githubgen/members.txt] [--allowlist cmd/githubgen/allowlist.txt] 
```

## Checking codeowners against OpenTelemetry membership via Github API

The optional `--check` argument can be passed to test the codeowners against the Github API.

This argument will interrogate the github API. To authenticate, set the environment variable `GITHUB_TOKEN` to a PAT token.

For each codeowner, the script will check if the user is registered as a member of the OpenTelemetry organization.

If any codeowner is missing, it will stop and print names of missing codeowners.

These can be added to allowlist.txt as a workaround.

On success, the script will print the codeowners to members.txt. This file is the cache used if `--check` is not used.

