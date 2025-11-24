#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

contrib_dir="$(dirname -- "$0")/../../.."
initial_checkout_size=$(du -BM -s "$contrib_dir" | sed 's/[^0-9]//g')
space_before_cleanup=$(df -BM --output=avail / | sed '1d;s/[^0-9]//g')

# The Android SDK is the biggest culprit for the lack of disk space in CI.
# It is installed into /usr/local/lib/android manually (ie. not with apt) by this script:
# https://github.com/actions/runner-images/blob/main/images/ubuntu/scripts/build/install-android-sdk.sh
# so let's delete the directory manually.
echo "Deleting unused Android SDK and tools..."
sudo rm -rf /usr/local/lib/android

free_space=$(df -BM --output=avail / | sed '1d;s/[^0-9]//g')
echo "Freed $((free_space - space_before_cleanup)) MiB of disk space."

# Hypothetical free space with the cleanup but without checkout
echo "BASELINE_SPACE=$((free_space + initial_checkout_size))" >> "$GITHUB_ENV"
