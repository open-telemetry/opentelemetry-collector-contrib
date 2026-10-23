#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
set -e

MONGO_CMD="mongosh"
if ! command -v mongosh &> /dev/null; then
    MONGO_CMD="mongo"
fi

setup() {
  if [ "$MONGO_CMD" = "mongosh" ]; then
    mongosh --eval "
      db.adminCommand({ profile: 0, slowms: 0 });
      use('testdb');
      db.createCollection('orders');
      for (let i = 0; i < 500; i++) { db.orders.insertOne({ status: 'pending', value: i }); }
    "
  else
    mongo <<EOF
    db.adminCommand({ profile: 0, slowms: 0 })
    use testdb
    db.createCollection("orders")
    for (let i = 0; i < 500; i++) { db.orders.insertOne({ status: 'pending', value: i }); }
EOF
  fi
}

echo "Setting up top_query test environment..."
end=$((SECONDS+20))
while [ $SECONDS -lt $end ]; do
  if setup; then
    echo "Setup complete!"
    exit 0
  fi
  echo "Retrying in 5 seconds..."
  sleep 5
done

echo "Setup failed"
exit 1
