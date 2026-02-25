#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

${COLLECTOR_BIN} "$@" | grep -v "service.AllowNoPipelines"
