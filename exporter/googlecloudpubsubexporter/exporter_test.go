// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter

import (
	"context"
	"testing"
	"time"

	pb "cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/api/option"
)

func TestName(t *testing.T) {
	exporter := &pubsubExporter{}
	assert.Equal(t, "googlecloudpubsub", exporter.Name())
}

func TestGenerateClientOptions(t *testing.T) {
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	exporterConfig := cfg.(*Config)
	exporterConfig.Endpoint = srv.Addr
	exporterConfig.UserAgent = "test-user-agent"
	exporterConfig.Insecure = true
	exporterConfig.ProjectID = "my-project"
	exporterConfig.Topic = "projects/my-project/topics/otlp"
	exporterConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 12 * time.Second,
	}
	exporter := ensureExporter(exportertest.NewNopCreateSettings(), exporterConfig)

	options := exporter.generateClientOptions()
	assert.Equal(t, option.WithUserAgent("test-user-agent"), options[0])

	exporter.config.Insecure = false
	options = exporter.generateClientOptions()
	assert.Equal(t, option.WithUserAgent("test-user-agent"), options[0])
	assert.Equal(t, option.WithEndpoint(srv.Addr), options[1])
}

func TestExporterDefaultSettings(t *testing.T) {
	ctx := context.Background()
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	_, err := srv.GServer.CreateTopic(ctx, &pb.Topic{
		Name: "projects/my-project/topics/otlp",
	})
	assert.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	exporterConfig := cfg.(*Config)
	exporterConfig.Endpoint = srv.Addr
	exporterConfig.Insecure = true
	exporterConfig.ProjectID = "my-project"
	exporterConfig.Topic = "projects/my-project/topics/otlp"
	exporterConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 12 * time.Second,
	}
	exporter := ensureExporter(exportertest.NewNopCreateSettings(), exporterConfig)
	assert.NoError(t, exporter.start(ctx, nil))
	assert.NoError(t, exporter.consumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, exporter.consumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, exporter.consumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, exporter.shutdown(ctx))
}

func TestExporterCompression(t *testing.T) {
	ctx := context.Background()
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	_, err := srv.GServer.CreateTopic(ctx, &pb.Topic{
		Name: "projects/my-project/topics/otlp",
	})
	assert.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	exporterConfig := cfg.(*Config)
	exporterConfig.Endpoint = srv.Addr
	exporterConfig.UserAgent = "test-user-agent"
	exporterConfig.Insecure = true
	exporterConfig.ProjectID = "my-project"
	exporterConfig.Topic = "projects/my-project/topics/otlp"
	exporterConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 12 * time.Second,
	}
	exporterConfig.Compression = "gzip"
	exporter := ensureExporter(exportertest.NewNopCreateSettings(), exporterConfig)
	assert.NoError(t, exporter.start(ctx, nil))
	assert.NoError(t, exporter.consumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, exporter.consumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, exporter.consumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, exporter.shutdown(ctx))
}
