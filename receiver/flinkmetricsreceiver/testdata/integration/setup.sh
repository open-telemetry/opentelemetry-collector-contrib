#!/usr/bin/env bash

set -e

./usr/local/flink/bin/flink run --detached /usr/local/flink/examples/streaming/StateMachineExample.jar

exit 0
