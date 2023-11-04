// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

// TODO: actual tests
func TestExample(t *testing.T) {
	// Create Connector
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	p := newProcessor(zaptest.NewLogger(t), cfg)

	// Fake consumer
	sink := &consumertest.TracesSink{}
	p.tracesConsumer = sink

	// Create Context
	ctx := metadata.NewIncomingContext(context.Background(), nil)
	require.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() { require.NoError(t, p.Shutdown(ctx)) }()

	// traces
	tracesBefore, err := golden.ReadTraces(filepath.Join("testdata", "traces", "traceBefore.yaml"))
	require.NoError(t, err)
	tracesAfter, err := golden.ReadTraces(filepath.Join("testdata", "traces", "traceAfter.yaml"))
	assert.NoError(t, err)
	// traces := buildSampleTrace()
	err = p.ConsumeTraces(ctx, tracesBefore)
	assert.NoError(t, err)
	// golden.WriteTraces(t, filepath.Join("testdata", "metrics", "output.yaml"), sink.AllTraces()[0])
	require.Equal(t, tracesAfter, sink.AllTraces()[0])
}

func TestCapabilitiesMutatesData(t *testing.T) {
	// Create an instance of connectorImp
	c := &processorImp{} // Assuming connectorImp is the type you provided

	// Call the Capabilities method
	capabilities := c.Capabilities()

	// Check if MutatesData is true
	if !capabilities.MutatesData {
		t.Errorf("Expected MutatesData to be true, but got false")
	}
}
