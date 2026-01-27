#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

declare BASELINE_SPACE

free_space=$(df -BM --output=avail / | sed '1d;s/[^0-9]//g')
used_space=$((BASELINE_SPACE - free_space))
echo "Job used $used_space MiB of disk space, $free_space MiB remain."
if [ "$used_space" -gt 14336 ]; then
  echo "WARNING: The amount of space used exceeds the 14 GiB guaranteed by GitHub."
fi
if [ "$free_space" -lt 1024 ]; then
  echo "WARNING: The amount of space remaining is dangerously low."
  # TODO: Make this warning more visible
fi
