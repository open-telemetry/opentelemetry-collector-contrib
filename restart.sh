#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#

set -ex

make docker-otelcontribcol
docker-compose up -d
docker restart otel
