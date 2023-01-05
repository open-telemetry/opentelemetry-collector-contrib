// Copyright The OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"
)

const (
	typeStr           = "awsfirehose"
	stability         = component.StabilityLevelAlpha
	defaultRecordType = cwmetricstream.TypeStr
	defaultEndpoint   = "0.0.0.0:4433"
)

var (
	errUnrecognizedRecordType = errors.New("unrecognized record type")
	availableRecordTypes      = map[string]bool{
		cwmetricstream.TypeStr: true,
	}
)

// NewFactory creates a receiver factory for awsfirehose. Currently, only
// available in metrics pipelines.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

// validateRecordType checks the available record types for the
// passed in one and returns an error if not found.
func validateRecordType(recordType string) error {
	if _, ok := availableRecordTypes[recordType]; !ok {
		return errUnrecognizedRecordType
	}
	return nil
}

// defaultMetricsUnmarshalers creates a map of the available metrics
// unmarshalers.
func defaultMetricsUnmarshalers(logger *zap.Logger) map[string]unmarshaler.MetricsUnmarshaler {
	cwmsu := cwmetricstream.NewUnmarshaler(logger)
	return map[string]unmarshaler.MetricsUnmarshaler{
		cwmsu.Type(): cwmsu,
	}
}

// createDefaultConfig creates a default config with the endpoint set
// to port 8443 and the record type set to the CloudWatch metric stream.
func createDefaultConfig() component.Config {
	return &Config{
		RecordType: defaultRecordType,
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: defaultEndpoint,
		},
	}
}

// createMetricsReceiver implements the CreateMetricsReceiver function type.
func createMetricsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	return newMetricsReceiver(cfg.(*Config), set, defaultMetricsUnmarshalers(set.Logger), nextConsumer)
}
