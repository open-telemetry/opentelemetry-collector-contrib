#!/bin/zsh
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

VERSION=`echo $1 | awk -F "/" '{print $NF}'`
echo "Using version ${VERSION}"

# check the version matches expected versioning e.g
# 0.6, 0.6b, 0.6b0, 0.6.0
if [[ ! "${VERSION}" =~ ^([0-9])(\.*[0-9]{1,5}[a-b]*){1,3}$ ]]; then
    echo "Version number invalid: $VERSION"
    exit 1
fi

# create the release branch
git checkout -b release/${VERSION}
git push origin release/${VERSION}

# create a temporary branch to create a PR for updated version and changelogs
git checkout -b release/${VERSION}-auto
./scripts/eachdist.py release --version ${VERSION}
rc=$?
if [ $rc != 0 ]; then
    echo "::set-output name=version_updated::0"
    exit 0
fi

git add **/version.py **/setup.cfg **/CHANGELOG.md

git commit -m "updating changelogs and version to ${VERSION}"

echo "Time to create a release, here's a sample title:"
echo "[pre-release] Update changelogs, version [${VERSION}]"
