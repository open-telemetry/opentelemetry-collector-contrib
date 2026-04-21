#!/bin/bash -ex

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

touch release-notes.md
echo "The OpenTelemetry Collector Contrib contains everything in the [opentelemetry-collector release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/$RELEASE_TAG), be sure to check the release notes there as well." >> release-notes.md
echo "" >> release-notes.md
echo "## End User Changelog" >> release-notes.md

awk '/<!-- next version -->/,/<!-- previous-version -->/' CHANGELOG.md > tmp-chlog.md # select changelog of latest version only
sed '1,3d' tmp-chlog.md >> release-notes.md # delete first 3 lines of file

echo "" >> release-notes.md
echo "## API Changelog" >> release-notes.md

awk '/<!-- next version -->/,/<!-- previous-version -->/' CHANGELOG-API.md > tmp-chlog-api.md # select changelog of latest version only
sed '1,3d' tmp-chlog-api.md >> release-notes.md # delete first 3 lines of file
