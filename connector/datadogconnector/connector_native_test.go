// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector

import (
	"context"
	"testing"
	"time"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var _ component.Component = (*traceToMetricConnectorNative)(nil) // testing that the connectorImp properly implements the type Component interface

// create test to create a connector, check that basic code compiles
func TestNewConnectorNative(t *testing.T) {
	err := featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), true)
	assert.NoError(t, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), false)
	}()

	factory := NewFactory()

	creationParams := connectortest.NewNopSettings()
	cfg := factory.CreateDefaultConfig().(*Config)

	tconn, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := tconn.(*traceToMetricConnectorNative)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}

func TestTraceToTraceConnectorNative(t *testing.T) {
	err := featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), true)
	assert.NoError(t, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), false)
	}()

	factory := NewFactory()

	creationParams := connectortest.NewNopSettings()
	cfg := factory.CreateDefaultConfig().(*Config)

	tconn, err := factory.CreateTracesToTraces(context.Background(), creationParams, cfg, consumertest.NewNop())
	assert.NoError(t, err)

	_, ok := tconn.(*traceToTraceConnector)
	assert.True(t, ok) // checks if the created connector implements the connectorImp struct
}

func creteConnectorNative(t *testing.T) (*traceToMetricConnectorNative, *consumertest.MetricsSink) {
	err := featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), true)
	assert.NoError(t, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), false)
	}()

	factory := NewFactory()

	creationParams := connectortest.NewNopSettings()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Traces.ResourceAttributesAsContainerTags = []string{semconv.AttributeCloudAvailabilityZone, semconv.AttributeCloudRegion, "az"}

	metricsSink := &consumertest.MetricsSink{}

	tconn, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, metricsSink)
	assert.NoError(t, err)

	connector, ok := tconn.(*traceToMetricConnectorNative)
	require.True(t, ok)
	return connector, metricsSink
}

func TestContainerTagsNative(t *testing.T) {
	connector, metricsSink := creteConnectorNative(t)
	err := connector.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		_ = connector.Shutdown(context.Background())
	}()

	trace1 := generateTrace()

	err = connector.ConsumeTraces(context.Background(), trace1)
	assert.NoError(t, err)

	// Send two traces to ensure unique container tags are added to the cache
	trace2 := generateTrace()
	err = connector.ConsumeTraces(context.Background(), trace2)
	assert.NoError(t, err)

	for {
		if len(metricsSink.AllMetrics()) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// check if the container tags are added to the metrics
	metrics := metricsSink.AllMetrics()
	assert.Equal(t, 1, len(metrics))

	ch := make(chan []byte, 100)
	tr := newTranslatorWithStatsChannel(t, zap.NewNop(), ch)
	_, err = tr.MapMetrics(context.Background(), metrics[0], nil)
	require.NoError(t, err)
	msg := <-ch
	sp := &pb.StatsPayload{}

	err = proto.Unmarshal(msg, sp)
	require.NoError(t, err)

	tags := sp.Stats[0].Tags
	assert.Equal(t, 3, len(tags))
	assert.ElementsMatch(t, []string{"region:my-region", "zone:my-zone", "az:my-az"}, tags)
}
