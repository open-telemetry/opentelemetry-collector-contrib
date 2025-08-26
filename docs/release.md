# opentelemetry-collector-contrib Release Procedure

This document describes the steps to follow when doing a new release of the
opentelemetry-collector-contrib repository. This release depends on the release of the
opentelemetry-collector repository, which is described in the [opentelemetry-collector release
procedure][1] document.

For general information about all Collector repositories release procedures, see the
[opentelemetry-collector release process documentation][1].

## Releasing opentelemetry-collector-contrib

0. Ensure that the [opentelemetry-collector release procedure][1] has been followed and a new
   opentelemetry-collector version has been released. The opentelemetry-collector-contrib release
   should be done after the opentelemetry-collector release.
1. Run the ["Automation - Prepare
   Release"](https://github.com/open-telemetry/opentelemetry-collector-contrib/actions/workflows/prepare-release.yml)
   action. Enter the version numbers without a leading `v`.
   This creates a pull request to update the changelog and version numbers in the repo. **While this
   PR is open all merging in Contrib should be halted**.
   - To update the PR needs make the changes in a fork and PR those changes into the
     `prepare-release-prs/x` branch. You do not need to wait for the CI to pass in this prep-to-prep
     PR.
   -  🛑 **Do not move forward until this PR is merged.** 🛑

2. Check out main and ensure it has the "Prepare release" commit in your local copy by pulling in
   the latest from `open-telemetry/opentelemetry-collector-contrib`. Use this commit to create a
   branch named `release/<release-series>` (e.g. `release/v0.85.x`). Push the new branch to
   `open-telemetry/opentelemetry-collector-contrib`. Assuming your upstream remote is named
   `upstream`, you can try the following commands:
   - `git checkout main && git fetch upstream && git rebase upstream/main`
   - `git switch -c release/<release series>` # append the commit hash of the PR in the last step if
     it is not the head of mainline
   - `git push -u upstream release/<release series>`

3. Make sure you are on `release/<release-series>`. Tag all the module groups with the new release
   version by running:

   ⚠️ If you set your remote using `https` you need to include
   `REMOTE=https://github.com/open-telemetry/opentelemetry-collector-contrib.git` in each command.
   ⚠️

   - `make push-tags MODSET=contrib-base`

4. Wait for the new tag build to pass successfully. A new `v0.85.0` release should be automatically
   created on Github by now, with the description containing the changelog for the new release.

5. Add a section at the top of the release description listing the unmaintained components
   ([example](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.114.0)).
   The list of unmaintained components can be found by [searching for issues with the "unmaintained"
   label](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aissue%20state%3Aopen%20label%3Aunmaintained).

## Post-release steps

After the release is complete, the release manager should do the following steps:

1. Create an issue or update existing issues for each problem encountered throughout the release in
the opentelemetry-collector-contrib repository and label them with the `release:retro` label.
Communicate the list of issues to the core release manager.

## Bugfix releases

See the [opentelemetry-collector release procedure][1] document for the bugfix release criteria and
process.

## Release schedule and release manager rotation

See the [opentelemetry-collector release procedure][1] document for the release schedule and release
manager rotation.


[1]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/release.md
