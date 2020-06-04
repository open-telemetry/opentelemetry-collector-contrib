#!/bin/bash

set -e

function cov {
    if [ ${TOX_ENV_NAME:0:4} == "py34" ]
    then
        pytest \
            --ignore-glob=*/setup.py \
            --ignore-glob=ext/opentelemetry-ext-opentracing-shim/tests/testbed/* \
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
cov ext/opentelemetry-ext-datadog
cov ext/opentelemetry-ext-flask
cov ext/opentelemetry-ext-requests
cov ext/opentelemetry-ext-jaeger
cov ext/opentelemetry-ext-opentracing-shim
cov ext/opentelemetry-exporter-cloud-trace
cov ext/opentelemetry-ext-wsgi
cov ext/opentelemetry-ext-zipkin
cov docs/examples/opentelemetry-example-app

# aiohttp is only supported on Python 3.5+.
if [ ${PYTHON_VERSION_INFO[1]} -gt 4 ]; then
    cov ext/opentelemetry-ext-aiohttp-client
# ext-asgi is only supported on Python 3.5+.
if [ ${PYTHON_VERSION_INFO[1]} -gt 4 ]; then
    cov ext/opentelemetry-ext-asgi
fi

coverage report --show-missing
coverage xml
