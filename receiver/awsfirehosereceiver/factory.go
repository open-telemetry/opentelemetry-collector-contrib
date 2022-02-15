// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"
)

const (
	typeStr           = "awsfirehose"
	defaultRecordType = cwmetricstream.TypeStr
)

var (
	errUnrecognizedRecordType = errors.New("unrecognized record type")
	availableRecordTypes      = map[string]bool{
		cwmetricstream.TypeStr: true,
	}
)

// NewFactory creates a receiver factory. Currently, only available in metrics pipelines.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func isValidRecordType(recordType string) error {
	if _, ok := availableRecordTypes[recordType]; !ok {
		return errUnrecognizedRecordType
	}
	return nil
}

func defaultMetricsUnmarshalers(logger *zap.Logger) map[string]unmarshaler.MetricsUnmarshaler {
	cwmsu := cwmetricstream.NewUnmarshaler(logger)
	return map[string]unmarshaler.MetricsUnmarshaler{
		cwmsu.Type(): cwmsu,
	}
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		RecordType:       defaultRecordType,
	}
}

func createMetricsReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	return newMetricsReceiver(cfg.(*Config), set, defaultMetricsUnmarshalers(set.Logger), nextConsumer)
}
