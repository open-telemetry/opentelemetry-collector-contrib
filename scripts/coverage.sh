#!/bin/bash

set -e

function cov {
    pytest \
        --cov ${1} \
        --cov-append \
        --cov-branch \
        --cov-report='' \
        ${1}
}

PYTHON_VERSION=$(python -c 'import sys; print(".".join(map(str, sys.version_info[:3])))')
PYTHON_VERSION_INFO=(${PYTHON_VERSION//./ })

coverage erase

cov instrumentation/opentelemetry-instrumentation-flask
cov instrumentation/opentelemetry-instrumentation-requests
cov instrumentation/opentelemetry-instrumentation-wsgi
cov instrumentation/opentelemetry-instrumentation-aiohttp-client
cov instrumentation/opentelemetry-instrumentation-asgi


coverage report --show-missing
coverage xml
