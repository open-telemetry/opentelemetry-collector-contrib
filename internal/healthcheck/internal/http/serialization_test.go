// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

func TestToSerializableEventWithAttributes(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("error_msg", "connection refused")
	attrs.PutInt("retry_count", 3)

	ev := componentstatus.NewEvent(
		componentstatus.StatusRecoverableError,
		componentstatus.WithAttributes(attrs),
	)

	se := toSerializableEvent(ev, true)

	require.NotNil(t, se.Attributes)
	assert.Equal(t, "connection refused", se.Attributes["error_msg"])
	assert.Equal(t, int64(3), se.Attributes["retry_count"])
}

func TestToSerializableEventWithoutAttributes(t *testing.T) {
	ev := componentstatus.NewEvent(componentstatus.StatusOK)
	se := toSerializableEvent(ev, true)

	assert.Nil(t, se.Attributes)
}

func TestToSerializableStatusAttributesOnLeafOnly(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("scraper", "cpu")

	leafEvent := componentstatus.NewEvent(
		componentstatus.StatusRecoverableError,
		componentstatus.WithAttributes(attrs),
	)

	// Build a simple aggregate: pipeline with one component that has attributes
	aggStatus := &status.AggregateStatus{
		// Aggregated event (no attributes since aggregation doesn't propagate them)
		Event: componentstatus.NewEvent(componentstatus.StatusRecoverableError),
		ComponentStatusMap: map[string]*status.AggregateStatus{
			"receiver:hostmetrics": {
				Event: leafEvent,
			},
		},
	}

	sst := toSerializableStatus(aggStatus, &serializationOptions{})

	// The aggregated (parent) node should have no attributes
	assert.Nil(t, sst.Attributes)

	// The leaf component should have attributes
	leaf := sst.ComponentStatuses["receiver:hostmetrics"]
	require.NotNil(t, leaf)
	require.NotNil(t, leaf.Attributes)
	assert.Equal(t, "cpu", leaf.Attributes["scraper"])
}
