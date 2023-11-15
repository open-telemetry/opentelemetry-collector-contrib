#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

files=(
    bin/otelcontribcol_darwin_arm64
    bin/otelcontribcol_darwin_amd64
    bin/otelcontribcol_linux_arm64
    bin/otelcontribcol_linux_ppc64le
    bin/otelcontribcol_linux_amd64
    bin/otelcontribcol_linux_s390x
    bin/otelcontribcol_windows_amd64.exe
);
for f in "${files[@]}"
do
    if [[ ! -f $f ]]
    then
        echo "$f does not exist."
        echo "passed=false" >> $GITHUB_OUTPUT
        exit 1
    fi
done
echo "passed=true" >> $GITHUB_OUTPUT
