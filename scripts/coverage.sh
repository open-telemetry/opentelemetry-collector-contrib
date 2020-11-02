#!/bin/bash

set -e

function cov {
    if [ ${TOX_ENV_NAME:0:4} == "py34" ]
    then
        pytest \
            --ignore-glob=*/setup.py \
            --ignore-glob=instrumentation/opentelemetry-instrumentation-opentracing-shim/tests/testbed/* \
            --cov ${1} \
            --cov-append \
            --cov-branch \
            --cov-report='' \
            ${1}
    else
        pytest \
            --ignore-glob=*/setup.py \
            --cov ${1} \
            --cov-append \
            --cov-branch \
            --cov-report='' \
            ${1}
    fi
}

PYTHON_VERSION=$(python -c 'import sys; print(".".join(map(str, sys.version_info[:3])))')
PYTHON_VERSION_INFO=(${PYTHON_VERSION//./ })

coverage erase

cov opentelemetry-api
cov opentelemetry-sdk
cov exporter/opentelemetry-exporter-datadog
cov instrumentation/opentelemetry-instrumentation-flask
cov instrumentation/opentelemetry-instrumentation-requests
cov exporter/opentelemetry-exporter-jaeger
cov instrumentation/opentelemetry-instrumentation-opentracing-shim
cov instrumentation/opentelemetry-instrumentation-wsgi
cov exporter/opentelemetry-exporter-zipkin
cov docs/examples/opentelemetry-example-app

# aiohttp is only supported on Python 3.5+.
if [ ${PYTHON_VERSION_INFO[1]} -gt 4 ]; then
    cov instrumentation/opentelemetry-instrumentation-aiohttp-client
# ext-asgi is only supported on Python 3.5+.
if [ ${PYTHON_VERSION_INFO[1]} -gt 4 ]; then
    cov instrumentation/opentelemetry-instrumentation-asgi
fi

coverage report --show-missing
coverage xml
