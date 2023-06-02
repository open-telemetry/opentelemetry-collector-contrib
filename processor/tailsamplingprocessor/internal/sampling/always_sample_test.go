// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestEvaluate_AlwaysSample(t *testing.T) {
	filter := NewAlwaysSample(componenttest.NewNopTelemetrySettings())
	decision, err := filter.Evaluate(context.Background(), pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
		16}), newTraceStringAttrs(nil, "example", "value"))
	assert.Nil(t, err)
	assert.Equal(t, decision, Sampled)
}
