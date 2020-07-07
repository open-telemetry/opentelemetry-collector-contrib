#!/bin/bash

set -euxo pipefail

SCRIPT_DIR="$( cd "$( dirname ${BASH_SOURCE[0]} )" && pwd )"
REPO_DIR="$( cd "$SCRIPT_DIR/../../" && pwd )"
VERSION="${1:-}"
OTELCONTRIBCOL_PATH="${2:-"$REPO_DIR/bin/otelcontribcol_linux_amd64"}"

if [[ -z "$VERSION" ]]; then
    latest_tag="$( git describe --abbrev=0 --match v[0-9]* )"
    VERSION="${latest_tag}-post"
fi

fpm -s dir -t deb -n otel-collector-contrib -v ${VERSION#v} -f -p "$REPO_DIR/bin/" \
    --vendor "OpenTelemetry Community" \
    --maintainer "OpenTelemetry Community <cncf-opentelemetry-community@lists.cncf.io>" \
    --description "OpenTelemetry Collector Contrib" \
    --license "Apache 2.0" \
    --url "https://github.com/open-telemetry/opentelemetry-collector-contrib" \
    --architecture "x86_64" \
    --deb-dist "stable" \
    $OTELCONTRIBCOL_PATH=/usr/bin/otelcontribcol