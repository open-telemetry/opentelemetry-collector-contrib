// Copyright The OpenTelemetry Authors
// Copyright (c) 2023 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"testing"
)

func TestVerifyGoLeaksOnce(t *testing.T) {
	defer VerifyGoLeaksOnce(t)
}

func TestMain(m *testing.M) {
	VerifyGoLeaks(m)
}
