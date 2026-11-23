// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_env) && !omit_detector_env

package env

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
