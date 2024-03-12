// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awss3receiver

import (
	"go.uber.org/goleak"
	"testing"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
