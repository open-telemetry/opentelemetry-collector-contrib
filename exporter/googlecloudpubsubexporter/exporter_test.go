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

package googlecloudpubsubexporter

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/api/option"
	pb "google.golang.org/genproto/googleapis/pubsub/v1"
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
	exporterConfig.endpoint = srv.Addr
	exporterConfig.UserAgent = "test-user-agent"
	exporterConfig.insecure = true
	exporterConfig.ProjectID = "my-project"
	exporterConfig.Topic = "projects/my-project/topics/otlp"
	exporterConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 12 * time.Second,
	}
	exporter := ensureExporter(componenttest.NewNopExporterCreateSettings(), exporterConfig)

	options := exporter.generateClientOptions()
	assert.Equal(t, option.WithUserAgent("test-user-agent"), options[0])

	exporter.config.insecure = false
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
	exporterConfig.endpoint = srv.Addr
	exporterConfig.insecure = true
	exporterConfig.ProjectID = "my-project"
	exporterConfig.Topic = "projects/my-project/topics/otlp"
	exporterConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 12 * time.Second,
	}
	exporter := ensureExporter(componenttest.NewNopExporterCreateSettings(), exporterConfig)
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
	exporterConfig.endpoint = srv.Addr
	exporterConfig.UserAgent = "test-user-agent"
	exporterConfig.insecure = true
	exporterConfig.ProjectID = "my-project"
	exporterConfig.Topic = "projects/my-project/topics/otlp"
	exporterConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 12 * time.Second,
	}
	exporterConfig.Compression = "gzip"
	exporter := ensureExporter(componenttest.NewNopExporterCreateSettings(), exporterConfig)
	assert.NoError(t, exporter.start(ctx, nil))
	assert.NoError(t, exporter.consumeTraces(ctx, ptrace.NewTraces()))
	assert.NoError(t, exporter.consumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, exporter.consumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, exporter.shutdown(ctx))
}
