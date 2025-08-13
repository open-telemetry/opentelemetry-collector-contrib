// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package otelarrowexporter

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
)

func TestSendTracesWithMetadata(t *testing.T) {
	// Start an OTel-Arrow receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv, err := otelArrowTracesReceiverOnGRPCServer(ln, false)
	rcv.hasMetadata = true
	rcv.spanCountByMetadata = make(map[string]int)

	rcv.start()
	require.NoError(t, err, "Failed to start mock OTLP receiver")
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.Arrow.MaxStreamLifetime = 100 * time.Second
	cfg.QueueSettings.Enabled = false

	cfg.MetadataCardinalityLimit = 10
	cfg.MetadataKeys = []string{"key1", "key2"}
	set := exportertest.NewNopSettings(metadata.Type)
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	bg := context.Background()
	exp, err := factory.CreateTraces(bg, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	callCtxs := []context.Context{
		client.NewContext(context.Background(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"key1": {"first"},
				"key2": {"second"},
			}),
		}),
		client.NewContext(context.Background(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"key1": {"third"},
				"key2": {"fourth"},
			}),
		}),
	}

	expectByContext := make([]int, len(callCtxs))

	requestCount := 3
	spansPerRequest := 33
	for requestNum := 0; requestNum < requestCount; requestNum++ {
		td := testdata.GenerateTraces(spansPerRequest)
		spans := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
		for spanIndex := 0; spanIndex < spansPerRequest; spanIndex++ {
			spans.At(spanIndex).SetName(fmt.Sprintf("%d-%d", requestNum, spanIndex))
		}

		num := requestNum % len(callCtxs)
		expectByContext[num] += spansPerRequest
		go func(n int) {
			assert.NoError(t, exp.ConsumeTraces(callCtxs[n], td))
		}(num)
	}

	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() == int32(requestCount)
	}, 1*time.Second, 5*time.Millisecond)
	assert.Eventually(t, func() bool {
		return rcv.totalItems.Load() == int32(requestCount*spansPerRequest)
	}, 1*time.Second, 5*time.Millisecond)
	assert.Eventually(t, func() bool {
		rcv.mux.Lock()
		defer rcv.mux.Unlock()
		return len(callCtxs) == len(rcv.spanCountByMetadata)
	}, 1*time.Second, 5*time.Millisecond)

	for idx, ctx := range callCtxs {
		md := client.FromContext(ctx).Metadata
		key := fmt.Sprintf("%s|%s", md.Get("key1"), md.Get("key2"))
		require.Equal(t, expectByContext[idx], rcv.spanCountByMetadata[key])
	}
}

func TestDuplicateMetadataKeys(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetadataKeys = []string{"myTOKEN", "mytoken"}
	err := cfg.Validate()
	require.ErrorContains(t, err, "duplicate")
	require.ErrorContains(t, err, "mytoken")
}

func TestMetadataExporterCardinalityLimit(t *testing.T) {
	const cardLimit = 10
	// Start an OTel-Arrow receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv, err := otelArrowTracesReceiverOnGRPCServer(ln, false)
	rcv.hasMetadata = true
	rcv.spanCountByMetadata = make(map[string]int)

	rcv.start()
	require.NoError(t, err, "Failed to start mock OTLP receiver")
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: ln.Addr().String(),
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.Arrow.MaxStreamLifetime = 100 * time.Second

	// disable queue settings to allow for error backpropagation.
	cfg.QueueSettings.Enabled = false

	cfg.MetadataCardinalityLimit = cardLimit
	cfg.MetadataKeys = []string{"key1", "key2"}
	set := exportertest.NewNopSettings(metadata.Type)
	bg := context.Background()
	exp, err := factory.CreateTraces(bg, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	for requestNum := 0; requestNum < cardLimit; requestNum++ {
		td := testdata.GenerateTraces(1)
		ctx := client.NewContext(bg, client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"key1": {fmt.Sprint(requestNum)},
				"key2": {fmt.Sprint(requestNum)},
			}),
		})

		assert.NoError(t, exp.ConsumeTraces(ctx, td))
	}

	td := testdata.GenerateTraces(1)
	ctx := client.NewContext(bg, client.Info{
		Metadata: client.NewMetadata(map[string][]string{
			"key1": {"limit_exceeded"},
			"key2": {"limit_exceeded"},
		}),
	})

	// above the metadata cardinality limit.
	err = exp.ConsumeTraces(ctx, td)
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.ErrorContains(t, err, "too many")

	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() == int32(cardLimit)
	}, 1*time.Second, 5*time.Millisecond)
	assert.Eventually(t, func() bool {
		return rcv.totalItems.Load() == int32(cardLimit)
	}, 1*time.Second, 5*time.Millisecond)

	require.Len(t, rcv.spanCountByMetadata, cardLimit)
}
