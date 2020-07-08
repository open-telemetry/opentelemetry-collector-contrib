#!/bin/bash

set -euxo pipefail

SCRIPT_DIR="$( cd "$( dirname ${BASH_SOURCE[0]} )" && pwd )"
REPO_DIR="$( cd "$SCRIPT_DIR/../../../../../" && pwd )"
VERSION="${1:-}"
ARCH="${2:-"amd64"}"
OUTPUT_DIR="${3:-"$REPO_DIR/bin/"}"
OTELCONTRIBCOL_PATH="$REPO_DIR/bin/otelcontribcol_linux_$ARCH"

if [[ -z "$VERSION" ]]; then
    latest_tag="$( git describe --abbrev=0 --match v[0-9]* )"
    VERSION="${latest_tag}~post"
fi

mkdir -p "$OUTPUT_DIR"

fpm -s dir -t rpm -n otel-contrib-collector -v ${VERSION#v} -f -p "$OUTPUT_DIR" \
    --vendor "OpenTelemetry Community" \
    --maintainer "OpenTelemetry Community <cncf-opentelemetry-community@lists.cncf.io>" \
    --description "OpenTelemetry Contrib Collector" \
    --license "Apache 2.0" \
    --rpm-summary "OpenTelemetry Contrib Collector" \
    --url "https://github.com/open-telemetry/opentelemetry-collector-contrib" \
    --architecture "$ARCH" \
    $OTELCONTRIBCOL_PATH=/usr/bin/otelcontribcol