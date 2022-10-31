#!/bin/bash

PROM_VERSION=v2.39.0
PROTO_VERSION=v1.3.2

# SRC_DIR is from protoc perspective. ie its the destination for our checkouts/clones
SRC_DIR=opentelemetry/exporter/prometheus_remote_write/gen/
DST_DIR=../src/opentelemetry/exporter/prometheus_remote_write/gen/

#TODO:
# Check that black & protoc are installed properly
echo "Creating our destination directory"
mkdir -p ${SRC_DIR}/gogoproto

# Clone prometheus
echo "Grabbing Prometheus protobuf files"
git clone --filter=blob:none --sparse https://github.com/prometheus/prometheus.git
cd prometheus
git checkout ${PROM_VERSION}
git sparse-checkout set prompb
cd ..


# We also need gogo.proto which is in the protobuf Repo
# Could also try to pull this locally from the install location of protobuf
# but that will be harder in a platform agnostic way.
echo "Grabbing gogo.proto"
git clone --filter=blob:none --sparse https://github.com/gogo/protobuf.git
cd protobuf
git checkout ${PROTO_VERSION}
git sparse-checkout set /gogoproto/gogo.proto
cd ..

# Move the proto files into our structure
echo "Moving proto files to ${SRC_DIR}"
cp prometheus/prompb/remote.proto prometheus/prompb/types.proto ${SRC_DIR}
cp protobuf/gogoproto/gogo.proto ${SRC_DIR}/gogoproto/


# A bit of a hack, but we need to fix the imports to fit the python structure.
# using sed to find the 3 files and point them at each other using OUR structure
echo "Fixing imports"
sed -i 's/import "types.proto";/import "opentelemetry\/exporter\/prometheus_remote_write\/gen\/types.proto";/' ${SRC_DIR}/remote.proto
sed -i 's/import "gogoproto\/gogo.proto";/import "opentelemetry\/exporter\/prometheus_remote_write\/gen\/gogoproto\/gogo.proto";/' ${SRC_DIR}/remote.proto
sed -i 's/import "gogoproto\/gogo.proto";/import "opentelemetry\/exporter\/prometheus_remote_write\/gen\/gogoproto\/gogo.proto";/' ${SRC_DIR}/types.proto


# Cleanup the repos
echo "Removing clones..."
rm -rf protobuf prometheus

# Used libprotoc 3.21.1 & protoc 21.7
echo "Compiling proto files to Python"
protoc -I .  --python_out=../src ${SRC_DIR}/gogoproto/gogo.proto ${SRC_DIR}/remote.proto ${SRC_DIR}/types.proto

echo "Running formatting on the generated files"
../../../scripts/eachdist.py format --path $PWD/..
