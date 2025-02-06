// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs"

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"
)

// Sender submits logs to Datadog intake
type Sender struct {
	logger  *zap.Logger
	api     *datadogV2.LogsApi
	verbose bool // reports whether payload contents should be dumped when logging at debug level
}

// logsV2 is the key in datadog ServerConfiguration
// It is being used to customize the endpoint for Datadog intake based on exporter configuration
// https://github.com/DataDog/datadog-api-client-go/blob/be7e034424012c7ee559a2153802a45df73232ea/api/datadog/configuration.go#L308
const logsV2 = "v2.LogsApi.SubmitLog"

// NewSender creates a new Sender
func NewSender(endpoint string, logger *zap.Logger, hcs confighttp.ClientConfig, verbose bool, apiKey string) *Sender {
	cfg := datadog.NewConfiguration()
	logger.Info("Logs sender initialized", zap.String("endpoint", endpoint))
	cfg.OperationServers[logsV2] = datadog.ServerConfigurations{
		datadog.ServerConfiguration{
			URL: endpoint,
		},
	}
	cfg.HTTPClient = clientutil.NewHTTPClient(hcs)
	cfg.AddDefaultHeader("DD-API-KEY", apiKey)
	apiClient := datadog.NewAPIClient(cfg)
	return &Sender{
		api:     datadogV2.NewLogsApi(apiClient),
		logger:  logger,
		verbose: verbose,
	}
}

// SubmitLogs submits the logs contained in payload to the Datadog intake
func (s *Sender) SubmitLogs(ctx context.Context, payload []datadogV2.HTTPLogItem) error {
	if s.verbose {
		s.logger.Debug("Submitting logs", zap.Any("payload", payload))
	}
	var (
		tags, prevtags string                  // keeps track of the ddtags of log items for grouping purposes
		batch          []datadogV2.HTTPLogItem // stores consecutive log items to be submitted together
	)
	// Correctly sets apiSubmitLogRequest ddtags field based on tags from translator Transform method
	for i, p := range payload {
		tags = p.GetDdtags()
		if prevtags == tags || i == 0 {
			// Batches consecutive log items with the same tags to be submitted together
			batch = append(batch, p)
			prevtags = tags
			continue
		}
		if err := s.handleSubmitLog(ctx, batch, prevtags); err != nil {
			return err
		}
		batch = []datadogV2.HTTPLogItem{p}
		prevtags = tags
	}
	return s.handleSubmitLog(ctx, batch, tags)
}

func (s *Sender) handleSubmitLog(ctx context.Context, batch []datadogV2.HTTPLogItem, tags string) error {
	opts := *datadogV2.NewSubmitLogOptionalParameters().
		WithContentEncoding(datadogV2.CONTENTENCODING_GZIP).
		WithDdtags(tags)
	_, r, err := s.api.SubmitLog(ctx, batch, opts)
	if err != nil {
		if r != nil {
			b := make([]byte, 1024) // 1KB message max
			n, _ := r.Body.Read(b)  // ignore any error
			s.logger.Error("Failed to send logs", zap.Error(err), zap.String("msg", string(b[:n])), zap.String("status_code", r.Status))
			return err
		}
		// If response is nil keep an error retryable
		// Assuming this is a transient error (e.g. network/connectivity blip)
		return err
	}
	return nil
}
