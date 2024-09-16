// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/publisher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

const (
	routingKey     = "routing_key"
	connectionName = "connection_name"
)

func TestStartAndShutdown(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	pub := mockPublisher{}
	var pubFactory = func(publisher.DialConfig) (publisher.Publisher, error) {
		return &pub, nil
	}
	exporter := newRabbitmqExporter(cfg, exportertest.NewNopSettings().TelemetrySettings, pubFactory, newTLSFactory(cfg), routingKey, connectionName)

	err := exporter.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	pub.On("Close").Return(nil)
	err = exporter.shutdown(context.Background())
	require.NoError(t, err)

	pub.AssertExpectations(t)
}

func TestStart_UnknownMarshallerEncoding(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	pub := mockPublisher{}
	var pubFactory = func(publisher.DialConfig) (publisher.Publisher, error) {
		return &pub, nil
	}

	unknownExtensionID := component.NewID(component.MustNewType("invalid_encoding"))
	cfg.EncodingExtensionID = &unknownExtensionID
	exporter := newRabbitmqExporter(cfg, exportertest.NewNopSettings().TelemetrySettings, pubFactory, newTLSFactory(cfg), routingKey, connectionName)

	err := exporter.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, "unknown encoding \"invalid_encoding\"")

	err = exporter.shutdown(context.Background())
	require.NoError(t, err)
}

func TestStart_PublisherCreationErr(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	var pubFactory = func(publisher.DialConfig) (publisher.Publisher, error) {
		return nil, errors.New("simulating error creating publisher")
	}
	exporter := newRabbitmqExporter(cfg, exportertest.NewNopSettings().TelemetrySettings, pubFactory, newTLSFactory(cfg), routingKey, connectionName)

	err := exporter.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, "simulating error creating publisher")

	err = exporter.shutdown(context.Background())
	require.NoError(t, err)
}

func TestStart_TLSError(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	pub := mockPublisher{}
	var pubFactory = func(publisher.DialConfig) (publisher.Publisher, error) {
		return &pub, nil
	}
	tlsFactory := func(context.Context) (*tls.Config, error) {
		return nil, errors.New("simulating tls config error")
	}
	exporter := newRabbitmqExporter(cfg, exportertest.NewNopSettings().TelemetrySettings, pubFactory, tlsFactory, routingKey, connectionName)

	err := exporter.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, "simulating tls config error")

	err = exporter.shutdown(context.Background())
	require.NoError(t, err)
}

func TestPublishMetrics(t *testing.T) {
	pub, exporter := exporterForPublishing(t)

	pub.On("Publish", mock.Anything, mock.MatchedBy(func(message publisher.Message) bool {
		return message.RoutingKey == routingKey && len(message.Body) > 0 && message.Exchange == ""
	})).Return(nil)
	err := exporter.publishMetrics(context.Background(), testdata.GenerateMetricsOneMetric())

	require.NoError(t, err)
	pub.AssertExpectations(t)
}

func TestPublishTraces(t *testing.T) {
	pub, exporter := exporterForPublishing(t)

	pub.On("Publish", mock.Anything, mock.MatchedBy(func(message publisher.Message) bool {
		return message.RoutingKey == routingKey && len(message.Body) > 0 && message.Exchange == ""
	})).Return(nil)
	err := exporter.publishTraces(context.Background(), testdata.GenerateTracesOneSpan())

	require.NoError(t, err)
	pub.AssertExpectations(t)
}

func TestPublishLogs(t *testing.T) {
	pub, exporter := exporterForPublishing(t)

	pub.On("Publish", mock.Anything, mock.MatchedBy(func(message publisher.Message) bool {
		return message.RoutingKey == routingKey && len(message.Body) > 0 && message.Exchange == ""
	})).Return(nil)
	err := exporter.publishLogs(context.Background(), testdata.GenerateLogsOneLogRecord())

	require.NoError(t, err)
	pub.AssertExpectations(t)
}

func exporterForPublishing(t *testing.T) (*mockPublisher, *rabbitmqExporter) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	pub := mockPublisher{}
	var pubFactory = func(publisher.DialConfig) (publisher.Publisher, error) {
		return &pub, nil
	}
	exporter := newRabbitmqExporter(cfg, exportertest.NewNopSettings().TelemetrySettings, pubFactory, newTLSFactory(cfg), routingKey, connectionName)

	err := exporter.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	return &pub, exporter
}

type mockPublisher struct {
	mock.Mock
}

func (c *mockPublisher) Publish(ctx context.Context, message publisher.Message) error {
	args := c.Called(ctx, message)
	return args.Error(0)
}

func (c *mockPublisher) Close() error {
	args := c.Called()
	return args.Error(0)
}
