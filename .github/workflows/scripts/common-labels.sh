#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# Label name mapping shared by add-labels.sh and add-labels-discussions.sh.
# This file is meant to be sourced, not executed directly.

declare -A COMMON_LABELS
COMMON_LABELS["arm64"]="arm64"
COMMON_LABELS["good-first-issue"]="good first issue"
COMMON_LABELS["help-wanted"]="help wanted"
COMMON_LABELS["discussion-needed"]="discussion needed"
COMMON_LABELS["needs-triage"]="needs triage"
COMMON_LABELS["os:mac"]="os:mac"
COMMON_LABELS["os:windows"]="os:windows"
COMMON_LABELS["waiting-for-author"]="waiting for author"
COMMON_LABELS["waiting-for-code-owners"]="waiting-for-code-owners"
COMMON_LABELS["waiting-for-approvers"]="waiting-for-approvers"
COMMON_LABELS["bug"]="bug"
COMMON_LABELS["priority:p0"]="priority:p0"
COMMON_LABELS["priority:p1"]="priority:p1"
COMMON_LABELS["priority:p2"]="priority:p2"
COMMON_LABELS["priority:p3"]="priority:p3"
COMMON_LABELS["stale"]="Stale"
COMMON_LABELS["never-stale"]="never stale"
COMMON_LABELS["skip-changelog"]="Skip Changelog"
