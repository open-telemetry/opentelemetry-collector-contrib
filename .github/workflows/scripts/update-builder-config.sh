#!/bin/bash -ex

cp builder/lumigo-builder-config.yaml cmd/otelcontribcol/builder-config.yaml

readonly released_version_regexp='v([0-9]+)'

if [[ "${GITHUB_REF}" =~ $released_version_regexp ]]; then
    version="${BASH_REMATCH[1]}"
else
    version="dev-$(git rev-parse HEAD | head -c8)"
fi

yq e -i ".dist.version = \"${version}\"" cmd/otelcontribcol/builder-config.yaml

cat cmd/otelcontribcol/builder-config.yaml