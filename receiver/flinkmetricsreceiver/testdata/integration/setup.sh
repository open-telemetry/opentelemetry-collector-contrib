#!/usr/bin/env bash

set -e

./usr/local/flink/bin/flink run --detached /integration/StateMachineExample.jar

exit 0