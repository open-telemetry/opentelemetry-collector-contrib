// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/mocks"
)

func TestNewAerospikeReceiver_BadEndpoint(t *testing.T) {
	testCases := []struct {
		name     string
		endpoint string
		errMsg   string
	}{
		{
			name:     "no port",
			endpoint: "localhost",
			errMsg:   "missing port in address",
		},
		{
			name:     "no address",
			endpoint: "",
			errMsg:   "missing port in address",
		},
	}

	cs, err := consumer.NewMetrics(func(ctx context.Context, ld pmetric.Metrics) error { return nil })
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{Endpoint: tc.endpoint}
			receiver, err := newAerospikeReceiver(receiver.CreateSettings{}, cfg, cs)
			require.ErrorContains(t, err, tc.errMsg)
			require.Nil(t, receiver)
		})
	}
}

func TestScrape_CollectClusterMetrics(t *testing.T) {
	t.Parallel()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	now := pcommon.NewTimestampFromTime(time.Now().UTC())

	expectedMB := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings())
	rb := metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())

	require.NoError(t, expectedMB.RecordAerospikeNodeConnectionOpenDataPoint(now, "22", metadata.AttributeConnectionTypeClient))
	rb.SetAerospikeNodeName("BB990C28F270008")
	expectedMB.EmitForResource(metadata.WithResource(rb.Emit()))

	require.NoError(t, expectedMB.RecordAerospikeNamespaceMemoryFreeDataPoint(now, "45"))
	rb.SetAerospikeNamespace("test")
	rb.SetAerospikeNodeName("BB990C28F270008")
	expectedMB.EmitForResource(metadata.WithResource(rb.Emit()))

	require.NoError(t, expectedMB.RecordAerospikeNamespaceMemoryFreeDataPoint(now, "30"))
	rb.SetAerospikeNamespace("bar")
	rb.SetAerospikeNodeName("BB990C28F270008")
	expectedMB.EmitForResource(metadata.WithResource(rb.Emit()))

	require.NoError(t, expectedMB.RecordAerospikeNodeConnectionOpenDataPoint(now, "1", metadata.AttributeConnectionTypeClient))
	rb.SetAerospikeNodeName("BB990C28F270009")
	expectedMB.EmitForResource(metadata.WithResource(rb.Emit()))

	require.NoError(t, expectedMB.RecordAerospikeNamespaceMemoryUsageDataPoint(now, "128", metadata.AttributeNamespaceComponentData))
	rb.SetAerospikeNamespace("test")
	rb.SetAerospikeNodeName("BB990C28F270009")
	expectedMB.EmitForResource(metadata.WithResource(rb.Emit()))

	// require.NoError(t, expectedMB.RecordAerospikeNamespaceMemoryUsageDataPoint(now, "badval", metadata.AttributeNamespaceComponentData))
	// expectedMB.EmitForResource(metadata.WithAerospikeNamespace("bar"), metadata.WithAerospikeNodeName("BB990C28F270009"))

	initialClient := mocks.NewAerospike(t)
	initialClient.On("Info").Return(clusterInfo{
		"BB990C28F270008": metricsMap{
			"node":               "BB990C28F270008",
			"client_connections": "22",
		},
		"BB990C28F270009": metricsMap{
			"node":               "BB990C28F270009",
			"client_connections": "1",
		},
	}, nil)

	initialClient.On("NamespaceInfo").Return(namespaceInfo{
		"BB990C28F270008": map[string]map[string]string{
			"test": metricsMap{
				"name":            "test",
				"memory_free_pct": "45",
			},
			"bar": metricsMap{
				"name":            "bar",
				"memory_free_pct": "30",
			},
		},
		"BB990C28F270009": map[string]map[string]string{
			"test": metricsMap{
				"name":                   "test",
				"memory_used_data_bytes": "128",
			},
			"bar": metricsMap{
				"name":                   "bar",
				"memory_used_data_bytes": "badval",
			},
		},
	}, nil)

	initialClient.On("Close").Return(nil)

	clientFactory := func() (Aerospike, error) {
		return initialClient, nil
	}
	clientFactoryNeg := func() (Aerospike, error) {
		return nil, errors.New("connection timeout")
	}

	receiver := &aerospikeReceiver{
		clientFactory: clientFactory,
		rb:            metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
		mb:            metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		logger:        logger.Sugar(),
		config: &Config{
			CollectClusterMetrics: true,
		},
	}

	require.NoError(t, receiver.start(context.Background(), componenttest.NewNopHost()))

	actualMetrics, err := receiver.scrape(context.Background())
	require.EqualError(t, err, "failed to parse int64 for AerospikeNamespaceMemoryUsage, value was badval: strconv.ParseInt: parsing \"badval\": invalid syntax")

	expectedMetrics := expectedMB.Emit()
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))

	require.NoError(t, receiver.shutdown(context.Background()))

	initialClient.AssertExpectations(t)

	receiverConnErr := &aerospikeReceiver{
		clientFactory: clientFactoryNeg,
		mb:            metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		logger:        logger.Sugar(),
		config: &Config{
			CollectClusterMetrics: true,
		},
	}

	initialClient.AssertNumberOfCalls(t, "Close", 1)

	err = receiverConnErr.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	require.Equal(t, receiverConnErr.client, nil, "client should be set to nil because of connection error")
}
