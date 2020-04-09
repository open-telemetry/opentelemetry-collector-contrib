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

coverage report
coverage xml
