#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
set -e

USER="otelu"
PASS="otelp"
MONGO_INITDB_ROOT_USERNAME="otel"
MONGO_INITDB_ROOT_PASSWORD="otel"

setup_permissions() {
  mongo -u $MONGO_INITDB_ROOT_USERNAME -p $MONGO_INITDB_ROOT_PASSWORD<<EOF
  use admin
  db.createUser(
          {
              user: "${USER}",
              pwd: "${PASS}",
              roles: [ "clusterMonitor"]
          }
  );
EOF
}

echo "Configuring ${USER} permissions. . ."
end=$((SECONDS+20))
while [ $SECONDS -lt $end ]; do
    if setup_permissions; then
        echo "Permissions configured!"
        break
    fi
    echo "Trying again in 5 seconds. . ."
    sleep 5
done


add_collections() {
    mongo -u $MONGO_INITDB_ROOT_USERNAME -p $MONGO_INITDB_ROOT_PASSWORD<<EOF
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
