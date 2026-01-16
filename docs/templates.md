# Text templates for repetitive tasks

## New Triager Proposal

Replace `[FULL NAME]` and `[USERNAME]` with the appropriate values, and add a line to README.md for this user.

```
Title: [chore] Add [FULL NAME] as triager
Body:

This PR adds [FULL NAME](https://github.com/[USERNAME]) as a new triager to the project.

@open-telemetry/collector-contrib-approvers please review their involvement with the project below, and consider approving this PR in support.

* [Issues filed](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is:issue+author:[USERNAME])
* [PRs](https://github.com/open-telemetry/opentelemetry-collector-contrib/pulls?q=is:pr+author:[USERNAME])
* [PR comments](https://github.com/open-telemetry/opentelemetry-collector-contrib/pulls?q=commenter:[USERNAME])

```
