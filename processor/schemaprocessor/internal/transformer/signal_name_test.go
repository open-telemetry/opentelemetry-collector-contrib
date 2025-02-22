// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

func TestSpanEventSignalNameChangeTransformer(t *testing.T) {
	s := ptrace.NewSpan()
	s.Events().AppendEmpty().SetName("event.name")
	c := SpanEventSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(map[string]string{
		"event.name": "event_name",
	})}
	require.NoError(t, c.Do(migrate.StateSelectorApply, s))
	require.Equal(t, "event_name", s.Events().At(0).Name())
}

func TestMetricSignalNameChangeTransformer(t *testing.T) {
	s := pmetric.NewMetric()
	s.SetName("event.name")
	c := MetricSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(map[string]string{
		"event.name": "event_name",
	})}
	require.NoError(t, c.Do(migrate.StateSelectorApply, s))
	require.Equal(t, "event_name", s.Name())
}
