// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
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
			receiver, err := newAerospikeReceiver(component.ReceiverCreateSettings{}, cfg, cs)
			require.ErrorContains(t, err, tc.errMsg)
			require.Nil(t, receiver)
		})
	}
}

func TestScrapeNode(t *testing.T) {
	testCases := []struct {
		name        string
		setupClient func() *mocks.Aerospike
		expectedErr string
	}{
		{
			name: "error response",
			setupClient: func() *mocks.Aerospike {
				client := &mocks.Aerospike{}
				client.On("Info").Return(nil, as.ErrNetTimeout)
				return client
			},
			expectedErr: as.ErrNetTimeout.Error(),
		},
		{
			name: "empty response",
			setupClient: func() *mocks.Aerospike {
				client := &mocks.Aerospike{}
				client.On("Info").Return(map[string]string{}, nil)
				return client
			},
		},
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := tc.setupClient()
			cs, err := consumer.NewMetrics(func(ctx context.Context, ld pmetric.Metrics) error { return nil })
			require.NoError(t, err)

			receiver, err := newAerospikeReceiver(
				component.ReceiverCreateSettings{
					TelemetrySettings: component.TelemetrySettings{
						Logger: logger,
					},
				},
				&Config{Endpoint: "localhost:3000"},
				cs,
			)
			require.NoError(t, err)
			errs := &scrapererror.ScrapeErrors{}
			receiver.scrapeNode(client, pcommon.NewTimestampFromTime(time.Now().UTC()), errs)

			if tc.expectedErr != "" {
				assert.EqualError(t, errs.Combine(), tc.expectedErr)
			}
			client.AssertExpectations(t)
			require.Equal(t, 0, receiver.mb.Emit().MetricCount())
		})
	}
}

func TestScrape_CollectClusterMetrics(t *testing.T) {
	t.Parallel()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	now := pcommon.NewTimestampFromTime(time.Now().UTC())

	expectedMB := metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings(), component.NewDefaultBuildInfo())

	require.NoError(t, expectedMB.RecordAerospikeNodeConnectionOpenDataPoint(now, "22", metadata.AttributeConnectionTypeClient))
	expectedMB.EmitForResource(metadata.WithAerospikeNodeName("Primary Node"))

	require.NoError(t, expectedMB.RecordAerospikeNamespaceMemoryFreeDataPoint(now, "45"))
	expectedMB.EmitForResource(metadata.WithAerospikeNamespace("test"), metadata.WithAerospikeNodeName("Primary Node"))

	require.NoError(t, expectedMB.RecordAerospikeNodeConnectionOpenDataPoint(now, "1", metadata.AttributeConnectionTypeClient))
	expectedMB.EmitForResource(metadata.WithAerospikeNodeName("Secondary Node"))

	require.NoError(t, expectedMB.RecordAerospikeNamespaceMemoryUsageDataPoint(now, "128", metadata.AttributeNamespaceComponentData))
	expectedMB.EmitForResource(metadata.WithAerospikeNamespace("test"), metadata.WithAerospikeNodeName("Secondary Node"))

	initialClient := mocks.NewAerospike(t)
	initialClient.On("Info").Return(map[string]string{
		"node":               "Primary Node",
		"namespaces":         "test;bar",
		"services":           "localhost:3001;localhost:3002;invalid",
		"client_connections": "22",
	}, nil)
	initialClient.On("NamespaceInfo", "test").Return(map[string]string{
		"name":            "test",
		"memory_free_pct": "45",
	}, nil)

	initialClient.On("NamespaceInfo", "bar").Return(nil, errors.New("no such namespace"))
	initialClient.On("Close").Return()

	peerClient := mocks.NewAerospike(t)
	peerClient.On("Info").Return(map[string]string{
		"node":               "Secondary Node",
		"namespaces":         "test",
		"client_connections": "1",
	}, nil)

	peerClient.On("NamespaceInfo", "test").Return(map[string]string{
		"name":                   "test",
		"memory_used_data_bytes": "128",
	}, nil)
	peerClient.On("Close").Return()

	clientFactory := func(host string, port int) (aerospike, error) {
		switch fmt.Sprintf("%s:%d", host, port) {
		case "localhost:3000":
			return initialClient, nil
		case "localhost:3001":
			return peerClient, nil
		case "localhost:3002":
			return nil, errors.New("connection timeout")
		}

		return nil, errors.New("unexpected endpoint")
	}
	receiver := &aerospikeReceiver{
		host:          "localhost",
		port:          3000,
		clientFactory: clientFactory,
		mb:            metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings(), component.NewDefaultBuildInfo()),
		logger:        logger,
		config: &Config{
			CollectClusterMetrics: true,
		},
	}

	actualMetrics, err := receiver.scrape(context.Background())
	require.EqualError(t, err, "no such namespace; connection timeout; address invalid: missing port in address")

	expectedMetrics := expectedMB.Emit()
	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))

	initialClient.AssertExpectations(t)
	peerClient.AssertExpectations(t)
}
