// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetrytest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSink(t *testing.T) {
	sink := NewSenderSink()
	sink.Start(context.Background())
	sink.Stop()
	assert.EqualValues(t, 1, sink.StartCount.Load())
	assert.EqualValues(t, 1, sink.StopCount.Load())
}
