// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/conf"
	"github.com/DataDog/datadog-agent/pkg/logs/message"
	"github.com/DataDog/datadog-agent/pkg/logs/sources"
	"github.com/DataDog/datadog-agent/pkg/util/scrubber"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	logsmapping "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/logs"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"

	pkgLogsAgent "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs/agent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
)

// otelTag specifies a tag to be added to all logs sent from the Datadog exporter
const otelTag = "otel_source:datadog_exporter"

type logsExporter struct {
	params           exporter.CreateSettings
	cfg              *Config
	ctx              context.Context // ctx triggers shutdown upon cancellation
	scrubber         scrub.Scrubber  // scrubber scrubs sensitive information from error messages
	onceMetadata     *sync.Once
	sourceProvider   source.Provider
	metadataReporter *inframetadata.Reporter
	logsAgent        *pkgLogsAgent.Agent
	pipelineChan     chan *message.Message
}

// newLogsExporter creates a new instance of logsExporter
func newLogsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg *Config,
	onceMetadata *sync.Once,
	sourceProvider source.Provider,
	metadataReporter *inframetadata.Reporter,
) (*logsExporter, error) {
	c := conf.NewConfig("test", "DD", strings.NewReplacer(".", "_"))
	c.Set("api_key", os.Getenv("DD_API_KEY"))
	c.Set("site", "datadoghq.com")
	logsAgent := pkgLogsAgent.NewLogsAgent(params.Logger, c)
	err := logsAgent.Start(ctx)
	if err != nil {
		params.Logger.Error("Failed to create logs agent", zap.Error(err))
		return nil, err
	}
	pipelineChan := logsAgent.GetPipelineProvider().NextPipelineChan()

	return &logsExporter{
		params:           params,
		cfg:              cfg,
		ctx:              ctx,
		onceMetadata:     onceMetadata,
		scrubber:         scrub.NewScrubber(),
		sourceProvider:   sourceProvider,
		metadataReporter: metadataReporter,
		logsAgent:        logsAgent,
		pipelineChan:     pipelineChan,
	}, nil
}

// createConsumeLogsFunc returns an implementation of consumer.ConsumeLogsFunc
func createConsumeLogsFunc(logger *zap.Logger, logSource *sources.LogSource, logsAgentChannel chan *message.Message) func(context.Context, plog.Logs) error {

	return func(_ context.Context, ld plog.Logs) (err error) {
		defer func() {
			if err != nil {
				newErr, scrubbingErr := scrubber.ScrubString(err.Error())
				if scrubbingErr != nil {
					err = scrubbingErr
				} else {
					err = errors.New(newErr)
				}
			}
		}()

		rsl := ld.ResourceLogs()
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
					ddLog := logsmapping.Transform(log, res, logger)

					var tags []string
					if ddTags := ddLog.GetDdtags(); ddTags == "" {
						tags = []string{otelTag}
					} else {
						tags = append(strings.Split(ddTags, ","), otelTag)
					}
					// Tags are set in the message origin instead
					ddLog.Ddtags = nil
					service := ""
					if ddLog.Service != nil {
						service = *ddLog.Service
					}
					status := ddLog.AdditionalProperties["status"]
					if status == "" {
						status = message.StatusInfo
					}
					origin := message.NewOrigin(logSource)
					origin.SetTags(tags)
					origin.SetService(service)
					origin.SetSource(logSourceName)
					ddLog.SetMessage(fmt.Sprintf("LOG FROM LOGS EXPORTER: %v", ddLog.Message))

					content, err := ddLog.MarshalJSON()
					if err != nil {
						logger.Error("Error parsing log: " + err.Error())
					}

					// ingestionTs is an internal field used for latency tracking on the status page, not the actual log timestamp.
					ingestionTs := time.Now().UnixNano()
					message := message.NewMessage(content, origin, status, ingestionTs)

					logsAgentChannel <- message
				}
			}
		}

		return nil
	}
}
