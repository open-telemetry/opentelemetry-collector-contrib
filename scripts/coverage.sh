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


coverage erase

cov opentelemetry-api
cov opentelemetry-sdk
cov ext/opentelemetry-ext-flask
cov ext/opentelemetry-ext-requests
cov ext/opentelemetry-ext-jaeger
cov ext/opentelemetry-ext-opentracing-shim
cov ext/opentelemetry-ext-wsgi
cov ext/opentelemetry-ext-zipkin
cov docs/examples/opentelemetry-example-app

coverage report
coverage xml
