#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

declare BASELINE_SPACE

free_space=$(df -BM --output=avail / | sed '1d;s/[^0-9]//g')
used_space=$((BASELINE_SPACE - free_space))
echo "Job used $used_space MiB of disk space, $free_space MiB remain."
if [ "$used_space" -gt 14336 ]; then
  echo "WARNING: The amount of space used exceeds the 14 GiB guaranteed by Github."
fi
if [ "$free_space" -lt 15000 ]; then
  echo "WARNING: The amount of space remaining is dangerously low."
  echo "GITHUB_REPOSITORY_OWNER=$GITHUB_REPOSITORY_OWNER; GITHUB_REF=$GITHUB_REF; GITHUB_HEAD_REF=$GITHUB_HEAD_REF"
  #if [ "$GITHUB_REPOSITORY_OWNER" = "open-telemetry" ] && [ "$GITHUB_REF" = "refs/heads/main" ] then
    echo "Adding comment on tracking issue..."
    gh issue comment 44458 -b "Job $GITHUB_JOB in workflow $GITHUB_WORKFLOW ended with $free_space MiB of disk space"
  #fi
fi
