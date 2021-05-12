#!/bin/bash
#
# This script:
#   1. parses the version number from the branch name
#   2. updates version.py files to match that version
#   3. iterates through CHANGELOG.md files and updates any files containing
#      unreleased changes
#   4. sets the output variable 'version_updated' to determine whether
#      the github action to create a pull request should run. this allows
#      maintainers to merge changes back into the release branch without
#      triggering unnecessary pull requests
#

VERSION=$(./scripts/eachdist.py version --mode stable)-$(./scripts/eachdist.py version --mode prerelease)
echo "Using version ${VERSION}"


# create the release branch
git checkout -b release/${VERSION}
git push origin release/${VERSION}

./scripts/eachdist.py update_versions --versions stable,prerelease
rc=$?
if [ $rc != 0 ]; then
    echo "::set-output name=version_updated::0"
    exit 0
fi

git add .

git commit -m "updating changelogs and version to ${VERSION}"

echo "Time to create a release, here's a sample title:"
echo "[pre-release] Update changelogs, version [${VERSION}]"
