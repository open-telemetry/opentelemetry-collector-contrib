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

package gcloudpubsubexporter

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/pstest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	pb "google.golang.org/genproto/googleapis/pubsub/v1"
)

func TestName(t *testing.T) {
	exporter := &pubsubExporter{}
	assert.Equal(t, "gcloudpubsub", exporter.Name())
}

func TestGenerateClientOptions(t *testing.T) {
	//ctx := context.Background()
	// Start a fake server running locally.

	srv := pstest.NewServer()
	defer srv.Close()
	core, _ := observer.New(zap.WarnLevel)
	exporter := &pubsubExporter{
		instanceName: "dummy",
		logger:       zap.New(core),
		userAgent:    "test-user-agent",

		config: &Config{
			Endpoint:    srv.Addr,
			UseInsecure: true,
			ProjectID:   "my-project",
		},
	}
	_, err := exporter.generateClientOptions()
	assert.NoError(t, err)

	exporter.config.UseInsecure = false
	_, err = exporter.generateClientOptions()
	assert.NoError(t, err)
}

func TestStartExporter(t *testing.T) {
	ctx := context.Background()
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	_, _ = srv.GServer.CreateTopic(ctx, &pb.Topic{
		Name: "projects/my-project/topics/my-trace",
	})
	_, _ = srv.GServer.CreateTopic(ctx, &pb.Topic{
		Name: "projects/my-project/topics/my-metric",
	})
	_, _ = srv.GServer.CreateTopic(ctx, &pb.Topic{
		Name: "projects/my-project/topics/my-log",
	})

	core, _ := observer.New(zap.WarnLevel)
	exporter := &pubsubExporter{
		instanceName: "dummy",
		logger:       zap.New(core),
		userAgent:    "test-user-agent",

		config: &Config{
			Endpoint:    srv.Addr,
			UseInsecure: true,
			ProjectID:   "my-project",
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 12 * time.Second,
			},
		},
		tracesTopicName:  "my-trace",
		metricsTopicName: "my-metric",
		logsTopicName:    "my-log",
	}
	assert.NoError(t, exporter.Start(ctx, nil))

	assert.NoError(t, exporter.ConsumeTraces(ctx, pdata.NewTraces()))
	assert.NoError(t, exporter.ConsumeMetrics(ctx, pdata.NewMetrics()))
	assert.NoError(t, exporter.ConsumeLogs(ctx, pdata.NewLogs()))
	assert.Nil(t, exporter.Shutdown(ctx))
	assert.Nil(t, exporter.Shutdown(ctx))
}

func TestStartWithNoServer(t *testing.T) {
	ctx := context.Background()
	core, _ := observer.New(zap.WarnLevel)
	exporter := &pubsubExporter{
		instanceName: "dummy",
		logger:       zap.New(core),
		userAgent:    "test-user-agent",

		config: &Config{
			Endpoint:    "localhost:42",
			UseInsecure: true,
			ProjectID:   "my-project",
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 1 * time.Second,
			},
		},
	}
	exporter.tracesTopicName = "no-trace"
	assert.Error(t, exporter.Start(ctx, nil))
	exporter.tracesTopicName = ""
	exporter.metricsTopicName = "no-metric"
	assert.Error(t, exporter.Start(ctx, nil))
	exporter.metricsTopicName = ""
	exporter.logsTopicName = "no-log"
	assert.Error(t, exporter.Start(ctx, nil))
}

func TestStartWithNoTopic(t *testing.T) {
	ctx := context.Background()
	// Start a fake server running locally.
	srv := pstest.NewServer()
	defer srv.Close()
	core, _ := observer.New(zap.WarnLevel)
	exporter := &pubsubExporter{
		instanceName: "dummy",
		logger:       zap.New(core),
		userAgent:    "test-user-agent",

		config: &Config{
			Endpoint:    srv.Addr,
			UseInsecure: true,
			ProjectID:   "my-project",
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 12 * time.Second,
			},
		},
	}
	exporter.tracesTopicName = "no-trace"
	assert.Error(t, exporter.Start(ctx, nil))
	exporter.tracesTopicName = ""
	exporter.metricsTopicName = "no-metric"
	assert.Error(t, exporter.Start(ctx, nil))
	exporter.metricsTopicName = ""
	exporter.logsTopicName = "no-log"
	assert.Error(t, exporter.Start(ctx, nil))
}
