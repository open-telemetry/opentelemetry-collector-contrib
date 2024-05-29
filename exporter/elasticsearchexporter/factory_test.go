// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"test:9200"}
	})
	params := exportertest.NewNopCreateSettings()
	exporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.TODO()))
}

func TestFactory_CreateMetricsExporter_Fail(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	params := exportertest.NewNopCreateSettings()
	_, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	require.Error(t, err, "expected an error when creating a traces exporter")
}

func TestFactory_CreateTracesExporter_Fail(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	params := exportertest.NewNopCreateSettings()
	_, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	require.Error(t, err, "expected an error when creating a traces exporter")
}

func TestFactory_CreateLogsAndTracesExporterWithDeprecatedIndexOption(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"test:9200"}
		cfg.Index = "test_index"
	})
	params := exportertest.NewNopCreateSettings()
	logsExporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, logsExporter)
	require.NoError(t, logsExporter.Shutdown(context.TODO()))

	tracesExporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, tracesExporter)
	require.NoError(t, tracesExporter.Shutdown(context.TODO()))
}

func TestSetDefaultUserAgentHeader(t *testing.T) {
	t.Run("insert default user agent header into empty", func(t *testing.T) {
		factory := NewFactory()
		cfg := factory.CreateDefaultConfig().(*Config)
		setDefaultUserAgentHeader(cfg, component.BuildInfo{Description: "mock OpenTelemetry Collector", Version: "latest"})
		assert.Equal(t, len(cfg.Headers), 1)
		assert.Equal(t, strings.Contains(cfg.Headers[userAgentHeaderKey], "OpenTelemetry Collector"), true)
	})

	t.Run("ignore user agent header if configured", func(t *testing.T) {
		factory := NewFactory()
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.Headers = map[string]string{
			userAgentHeaderKey: "mock user agent header",
		}
		setDefaultUserAgentHeader(cfg, component.BuildInfo{Description: "mock OpenTelemetry Collector", Version: "latest"})
		assert.Equal(t, len(cfg.Headers), 1)
		assert.Equal(t, cfg.Headers[userAgentHeaderKey], "mock user agent header")
	})
}

func TestMakeBatchFuncs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	esClient, err := newElasticsearchClient(logger, cfg)
	require.NoError(t, err)
	bi, err := newBulkIndexer(logger, esClient, cfg)
	require.NoError(t, err)

	// creates bulk indexer items based on the count and offset. For a
	// given count and offset identical output will be generated. This
	// allows testing for merge operations assuming each batch is
	// identified by a sequence number.
	createRandomBulkIndexerItems := func(count, offset int) []bulkIndexerItem {
		ret := make([]bulkIndexerItem, 0, count)
		for i := offset; i < offset+count; i++ {
			tmp := fmt.Sprintf("test-%d", i)
			ret = append(ret, bulkIndexerItem{
				Index: tmp,
				Body:  []byte(tmp),
			})
		}
		return ret
	}
	_, batchMergeSplit := makeBatchFuncs(bi)

	for _, tc := range []struct {
		name     string
		maxItems int
		input1   []bulkIndexerItem
		input2   []bulkIndexerItem
		expected [][]bulkIndexerItem
	}{
		{
			name:     "merge_to_one",
			maxItems: 10,
			input1:   createRandomBulkIndexerItems(7, 0),
			input2:   createRandomBulkIndexerItems(3, 7),
			expected: [][]bulkIndexerItem{createRandomBulkIndexerItems(10, 0)},
		},
		{
			name:     "no_merge",
			maxItems: 10,
			input1:   createRandomBulkIndexerItems(7, 0),
			input2:   createRandomBulkIndexerItems(5, 0),
			expected: [][]bulkIndexerItem{
				createRandomBulkIndexerItems(7, 0),
				createRandomBulkIndexerItems(5, 0),
			},
		},
		{
			name:     "large_first_req",
			maxItems: 10,
			input1:   createRandomBulkIndexerItems(101, 0),
			input2:   createRandomBulkIndexerItems(5, 101),
			expected: [][]bulkIndexerItem{
				createRandomBulkIndexerItems(10, 0),
				createRandomBulkIndexerItems(10, 10),
				createRandomBulkIndexerItems(10, 20),
				createRandomBulkIndexerItems(10, 30),
				createRandomBulkIndexerItems(10, 40),
				createRandomBulkIndexerItems(10, 50),
				createRandomBulkIndexerItems(10, 60),
				createRandomBulkIndexerItems(10, 70),
				createRandomBulkIndexerItems(10, 80),
				createRandomBulkIndexerItems(10, 90),
				createRandomBulkIndexerItems(6, 100),
			},
		},
		{
			name:     "large_second_req",
			maxItems: 10,
			input1:   createRandomBulkIndexerItems(5, 0),
			input2:   createRandomBulkIndexerItems(101, 5),
			expected: [][]bulkIndexerItem{
				createRandomBulkIndexerItems(10, 0),
				createRandomBulkIndexerItems(10, 10),
				createRandomBulkIndexerItems(10, 20),
				createRandomBulkIndexerItems(10, 30),
				createRandomBulkIndexerItems(10, 40),
				createRandomBulkIndexerItems(10, 50),
				createRandomBulkIndexerItems(10, 60),
				createRandomBulkIndexerItems(10, 70),
				createRandomBulkIndexerItems(10, 80),
				createRandomBulkIndexerItems(10, 90),
				createRandomBulkIndexerItems(6, 100),
			},
		},
		{
			name:     "large_reqs",
			maxItems: 10,
			input1:   createRandomBulkIndexerItems(54, 0),
			input2:   createRandomBulkIndexerItems(55, 54),
			expected: [][]bulkIndexerItem{
				createRandomBulkIndexerItems(10, 0),
				createRandomBulkIndexerItems(10, 10),
				createRandomBulkIndexerItems(10, 20),
				createRandomBulkIndexerItems(10, 30),
				createRandomBulkIndexerItems(10, 40),
				createRandomBulkIndexerItems(10, 50),
				createRandomBulkIndexerItems(10, 60),
				createRandomBulkIndexerItems(10, 70),
				createRandomBulkIndexerItems(10, 80),
				createRandomBulkIndexerItems(10, 90),
				createRandomBulkIndexerItems(9, 100),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := batchMergeSplit(
				context.Background(),
				exporterbatcher.MaxSizeConfig{MaxSizeItems: tc.maxItems},
				&request{bulkIndexer: bi, Items: tc.input1},
				&request{bulkIndexer: bi, Items: tc.input2},
			)
			require.NoError(t, err)
			require.Equal(t, len(tc.expected), len(out))
			for i := 0; i < len(tc.expected); i++ {
				assert.Equal(t, tc.expected[i], out[i].(*request).Items)
			}
		})
	}
}
