#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

echo "Available disk space before:"
df -h /

echo "Deleting unused Android SDK and tools..."
sudo rm -rf /usr/local/lib/android

echo "Available disk space after:"
df -h /
