#!/bin/sh

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

if command -v systemctl >/dev/null 2>&1; then
    systemctl stop otel-contrib-collector.service
    systemctl disable otel-contrib-collector.service
fi