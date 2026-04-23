// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("github.com/twmb/franz-go/pkg/kfake.(*group).manage"))
}
