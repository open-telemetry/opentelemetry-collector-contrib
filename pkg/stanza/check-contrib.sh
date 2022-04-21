#!/bin/bash

set -e

CURDIR=$(pwd)
CONTRIB_PATH="/tmp/opentelemetry-collector-contrib"
LOG_COLLECTION_MOD_NAME="github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza"
LOG_COLLECTION_MODULES=$(find ${CONTRIB_PATH} ! -path ${CONTRIB_PATH}/go.mod ! -path ${CONTRIB_PATH}/cmd/configschema/go.mod -type f -name "go.mod" -exec grep -l ${LOG_COLLECTION_MOD_NAME} {} \; | xargs -L 1 dirname | sort)
echo "Log collection modules - ${LOG_COLLECTION_MODULES}"

for module in ${LOG_COLLECTION_MODULES}
do
  pushd ${module}
  go mod edit -replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza=${CURDIR}
  rm -fr go.sum
  go mod tidy -compat=1.17
  go test
  go mod edit -dropreplace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza
  popd
done
