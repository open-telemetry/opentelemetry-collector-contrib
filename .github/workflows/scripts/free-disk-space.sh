#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

contrib_dir="$(dirname -- "$0")/../../.."
initial_checkout_size=$(du -sm "$contrib_dir" 2>/dev/null | cut -f1 || echo "0")
space_before_cleanup=$(df -m / | tail -1 | awk '{print $4}')

# The Android SDK is the biggest culprit for the lack of disk space in CI.
# It is installed into /usr/local/lib/android manually (ie. not with apt) by this script:
# https://github.com/actions/runner-images/blob/main/images/ubuntu/scripts/build/install-android-sdk.sh

echo "Deleting unused Android SDK and tools..."
if [ -d "/usr/local/lib/android" ]; then
    sudo rm -rf /usr/local/lib/android
else
    echo "Android SDK directory not found, skipping cleanup"
fi

free_space=$(df -m / | tail -1 | awk '{print $4}')
freed_space=$((free_space - space_before_cleanup))
echo "Freed ${freed_space} MiB of disk space."

# Hypothetical free space with the cleanup but without checkout
baseline_space=$((free_space + initial_checkout_size))
echo "BASELINE_SPACE=${baseline_space}" >> "$GITHUB_ENV"
