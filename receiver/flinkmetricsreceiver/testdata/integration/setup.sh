#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

./usr/local/flink/bin/flink run --detached /usr/local/flink/examples/streaming/StateMachineExample.jar

exit 0
