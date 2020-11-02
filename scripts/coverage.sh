#!/bin/bash

set -e

function cov {
    pytest \
        --ignore-glob=*/setup.py \
        --cov ${1} \
        --cov-append \
        --cov-branch \
        --cov-report='' \
        ${1}
}

PYTHON_VERSION=$(python -c 'import sys; print(".".join(map(str, sys.version_info[:3])))')
PYTHON_VERSION_INFO=(${PYTHON_VERSION//./ })

coverage erase

cov exporter/opentelemetry-exporter-datadog
cov instrumentation/opentelemetry-instrumentation-flask
cov instrumentation/opentelemetry-instrumentation-requests
cov instrumentation/opentelemetry-instrumentation-wsgi

# aiohttp is only supported on Python 3.5+.
if [ ${PYTHON_VERSION_INFO[1]} -gt 4 ]; then
    cov instrumentation/opentelemetry-instrumentation-aiohttp-client
# ext-asgi is only supported on Python 3.5+.
if [ ${PYTHON_VERSION_INFO[1]} -gt 4 ]; then
    cov instrumentation/opentelemetry-instrumentation-asgi
fi

coverage report --show-missing
coverage xml
