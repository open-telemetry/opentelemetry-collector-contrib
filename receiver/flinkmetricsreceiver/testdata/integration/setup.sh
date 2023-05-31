#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

/opt/flink/bin/flink run --detached /opt/flink/examples/streaming/StateMachineExample.jar

exit 0
