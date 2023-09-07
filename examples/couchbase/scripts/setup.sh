#!/bin/sh

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
set -e

CONTAINER_NAME=${CONTAINER_NAME:-"couchbase_couchbase_1"}

# Wait until couchbase is responsive
until curl --silent http://127.0.0.1:8091/pools > /dev/null; do
  >&2 echo "Waiting for Couchbase Server to be available ..."
  sleep 1
done


echo "Initializing cluster..."
docker exec "$CONTAINER_NAME" couchbase-cli cluster-init -c 127.0.0.1 --cluster-username otelu --cluster-password otelpassword \
  --cluster-name otelc --cluster-ramsize 512 --cluster-index-ramsize 256 --services data,index,query,fts \
  --index-storage-setting default

echo "Creating buckets..."
docker exec "$CONTAINER_NAME" couchbase-cli bucket-create -c 127.0.0.1 --username otelu --password otelpassword --bucket-type couchbase --bucket-ramsize 256 --bucket test_bucket_1

docker exec "$CONTAINER_NAME" couchbase-cli bucket-create -c 127.0.0.1 --username otelu --password otelpassword --bucket-type couchbase --bucket-ramsize 256 --bucket test_bucket_2

echo "Creating users..."
docker exec "$CONTAINER_NAME" couchbase-cli user-manage -c 127.0.0.1:8091 -u otelu -p otelpassword --set --rbac-username sysadmin --rbac-password otelpassword \
 --rbac-name "sysadmin" --roles admin --auth-domain local

docker exec "$CONTAINER_NAME" couchbase-cli user-manage -c 127.0.0.1:8091 -u otelu -p otelpassword --set --rbac-username admin --rbac-password otelpassword \
 --rbac-name "admin" --roles bucket_full_access[*] --auth-domain local

# Need to wait until query service is ready to process N1QL queries
echo "Waiting for query service to be ready..."
sleep 20

# Create otelb indexes
echo "Create test_bucket_1 index"
docker exec "$CONTAINER_NAME" cbq -u otelu -p otelpassword -s "CREATE PRIMARY INDEX idx_primary ON \`test_bucket_1\`;"

# Create test_bucket indexes
echo "Create test_bucket_2 index"
docker exec "$CONTAINER_NAME" cbq -u otelu -p otelpassword -s "CREATE PRIMARY INDEX idx_primary ON \`test_bucket_2\`;"
