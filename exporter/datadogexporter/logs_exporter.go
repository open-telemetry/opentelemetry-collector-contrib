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

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"sync"

	"github.com/DataDog/datadog-agent/pkg/otlp/model/source"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils"
)

type logsExporter struct {
	params         component.ExporterCreateSettings
	cfg            *Config
	ctx            context.Context // ctx triggers shutdown upon cancellation
	scrubber       scrub.Scrubber  // scrubber scrubs sensitive information from error messages
	sender         *logs.Sender
	onceMetadata   *sync.Once
	sourceProvider source.Provider
}

// newLogsExporter creates a new instance of logsExporter
func newLogsExporter(ctx context.Context, params component.ExporterCreateSettings, cfg *Config, onceMetadata *sync.Once, sourceProvider source.Provider) (*logsExporter, error) {
	// create Datadog client
	// validation endpoint is provided by Metrics
	client := utils.CreateClient(cfg.API.Key, cfg.Metrics.TCPAddr.Endpoint)
	// validate the apiKey
	if err := utils.ValidateAPIKey(params.Logger, client); err != nil && cfg.API.FailOnInvalidKey {
		return nil, err
	}

	s := logs.NewSender(cfg.Logs.TCPAddr.Endpoint, params.Logger, cfg.TimeoutSettings, cfg.LimitedHTTPClientSettings.TLSSetting.InsecureSkipVerify, cfg.API.Key)

	return &logsExporter{
		params:         params,
		cfg:            cfg,
		ctx:            ctx,
		sender:         s,
		onceMetadata:   onceMetadata,
		scrubber:       scrub.NewScrubber(),
		sourceProvider: sourceProvider,
	}, nil
}

var _ consumer.ConsumeLogsFunc = (*logsExporter)(nil).consumeLogs

// consumeLogs is implementation of cosumer.ConsumeLogsFunc
func (exp *logsExporter) consumeLogs(ctx context.Context, ld plog.Logs) (err error) {
	defer func() { err = exp.scrubber.Scrub(err) }()
	if exp.cfg.HostMetadata.Enabled {
		// start host metadata with resource attributes from
		// the first payload.
		exp.onceMetadata.Do(func() {
			attrs := pcommon.NewMap()
			if ld.ResourceLogs().Len() > 0 {
				attrs = ld.ResourceLogs().At(0).Resource().Attributes()
			}
			go metadata.Pusher(exp.ctx, exp.params, newMetadataConfigfromConfig(exp.cfg), exp.sourceProvider, attrs)
		})
	}

	rsl := ld.ResourceLogs()
	var payload []datadogV2.HTTPLogItem
	// Iterate over resource logs
	for i := 0; i < rsl.Len(); i++ {
		rl := rsl.At(i)
		sls := rl.ScopeLogs()
		res := rl.Resource()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			lsl := sl.LogRecords()
			// iterate over Logs
			for k := 0; k < lsl.Len(); k++ {
				log := lsl.At(k)
				payload = append(payload, logs.Transform(log, res))
			}
		}
	}
	return exp.sender.SubmitLogs(exp.ctx, payload)
}
