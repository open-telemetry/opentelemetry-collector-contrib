#!/bin/bash -ex

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

TAG="${GITHUB_REF##*/}"
if [[ $TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+.* ]]
then
    echo "tag=$TAG" >> $GITHUB_OUTPUT
fi
