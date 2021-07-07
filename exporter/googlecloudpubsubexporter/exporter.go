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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

const name = "googlecloudpubsub"

type pubsubExporter struct {
	instanceName string
	logger       *zap.Logger

	topicName string

	//
	userAgent string
	ceSource  string
	config    *Config
	//
}

func (*pubsubExporter) Name() string {
	return name
}

func (ex *pubsubExporter) start(ctx context.Context, _ component.Host) error {
	return nil
}

func (ex *pubsubExporter) shutdown(context.Context) error {
	return nil
}

func (ex *pubsubExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (ex *pubsubExporter) consumeTraces(ctx context.Context, td pdata.Traces) error {
	return nil
}

func (ex *pubsubExporter) consumeMetrics(ctx context.Context, td pdata.Metrics) error {
	return nil
}

func (ex *pubsubExporter) consumeLogs(ctx context.Context, td pdata.Logs) error {
	return nil
}
