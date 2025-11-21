#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

# Depends on variables defined in free-disk-space.sh

get_free_space
used_space=$((initial_space - free_space - extra_space))
echo "Job used $used_space MiB of disk space, $free_space MiB remain."
if [ "$used_space" -gt 14336 ]; then
  echo "WARNING: The amount used exceeds the guaranteed 14 GiB of free space."
  exit 1
fi
