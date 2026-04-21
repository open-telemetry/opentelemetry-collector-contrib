#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
set -e

add_collections() {
    mongo <<EOF
    use testdb
    db.createCollection("orders")
EOF
}

echo "Adding collections. . ."
end=$((SECONDS+20))
while [ $SECONDS -lt $end ]; do
    if add_collections; then
        echo "collections added!"
        exit 0
    fi
    echo "Trying again in 5 seconds. . ."
    sleep 5
done

echo "Failed to add collections"
