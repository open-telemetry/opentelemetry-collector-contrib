# Releasing OpenTelemetry Packages (for maintainers only)
This document explains how to publish all OT modules at version x.y.z. Ensure that you’re following semver when choosing a version number.

Release Process:
* [Checkout a clean repo](#checkout-a-clean-repo)
* [Update versions](#update-versions)
* [Create a new branch](#create-a-new-branch)
* [Open a Pull Request](#open-a-pull-request)
* [Create a Release](#Create-a-Release)
* [Move stable tag](#Move-stable-tag)
* [Update main](#Update-main)
* [Check PyPI](#Check-PyPI)
* [Troubleshooting](#troubleshooting)

## Checkout a clean repo
To avoid pushing untracked changes, check out the repo in a new dir

## Update versions
The update of the version information relies on the information in eachdist.ini to identify which packages are stable, prerelease or
experimental. Update the desired version there to begin the release process.

## Create a new branch
The following script does the following:
- update main locally
- creates a new release branch `release/<version>`
- updates version and changelog files
- commits the change

*NOTE: This script was run by a GitHub Action but required the Action bot to be excluded from the CLA check, which it currently is not.*

```bash
./scripts/prepare_release.sh
```

## Open a Pull Request

The PR should be opened from the `release/<version>` branch created as part of running `prepare_release.sh` in the steps above.

## Create a Release

- Create the GH release from the main branch, using a new tag for this micro version, e.g. `v0.7.0`
- Copy the changelogs from all packages that changed into the release notes (and reformat to remove hard line wraps)


## Check PyPI

This should be handled automatically on release by the [publish action](https://github.com/open-telemetry/opentelemetry-python/blob/main/.github/workflows/publish.yml).

- Check the [action logs](https://github.com/open-telemetry/opentelemetry-python/actions?query=workflow%3APublish) to make sure packages have been uploaded to PyPI
- Check the release history (e.g. https://pypi.org/project/opentelemetry-api/#history) on PyPI

If for some reason the action failed, see [Publish failed](#publish-failed) below

## Move stable tag

This will ensure the docs are pointing at the stable release.

```bash
git tag -d stable
git tag stable
git push --delete origin tagname
git push origin stable
```

To validate this worked, ensure the stable build has run successfully: https://readthedocs.org/projects/opentelemetry-python/builds/. If the build has not run automatically, it can be manually trigger via the readthedocs interface.

## Update main

Ensure the version and changelog updates have been applied to main. Update the versions in eachdist.ini once again this time to include the `.dev0` tag and
run eachdist once again:
```bash
./scripts/eachdist.py update_versions --versions stable,prerelease
```

If the diff includes significant changes, create a pull request to commit the changes and once the changes are merged, click the "Run workflow" button for the Update [OpenTelemetry Website Docs](https://github.com/open-telemetry/opentelemetry-python/actions/workflows/docs-update.yml) GitHub Action.

## Hotfix procedure

A `hotfix` is defined as a small change developed to correct a bug that should be released as quickly as possible. Due to the nature of hotfixes, they usually will only affect one or a few packages. Therefore, it usually is not necessary to go through the entire release process outlined above for hotfixes. Follow the below steps how to release a hotfix:

1. Identify the packages that are affected by the bug. Make the changes to those packages, merging to `main`, as quickly as possible.
2. On your local machine, remove the `dev0` tags from the version number and increment the patch version number.
3. On your local machine, update `CHANGELOG.md` with the date of the hotfix change.
4. With administrator privileges for PyPi, manually publish the affected packages.
    a. Install [twine](https://pypi.org/project/twine/)
    b. Navigate to where the `setup.py` file exists for the package you want to publish.
    c. Run `python setup.py sdist bdist_wheel`. You may have to install [wheel](https://pypi.org/project/wheel/) as well.
    d. Validate your built distributions by running `twine check dist/*`.
    e. Upload distributions to PyPi by running `twine upload dist/*`.
5. Note that since hotfixes are manually published, the build scripts for publish after creating a release are not run.

## Troubleshooting

### Publish failed

If for some reason the action failed, do it manually:

- Switch to the release branch (important so we don't publish packages with "dev" versions)
- Build distributions with `./scripts/build.sh`
- Delete distributions we don't want to push (e.g. `testutil`)
- Push to PyPI as `twine upload --skip-existing --verbose dist/*`
- Double check PyPI!