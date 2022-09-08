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

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils"
)

// Sender is wrapper for LogsApi from datadog-api-client-go which buffers the logs in memory
type Sender struct {
	logger *zap.Logger
	api    *datadogV2.LogsApi
	buffer []datadogV2.HTTPLogItem
}

// logsV2 is the key in server configuration
// https://github.com/DataDog/datadog-api-client-go/blob/be7e034424012c7ee559a2153802a45df73232ea/api/datadog/configuration.go#L308
const logsV2 = "v2.LogsApi.SubmitLog"

// submitOpts would enable sending gzip logs
var submitOpts = *datadogV2.NewSubmitLogOptionalParameters().WithContentEncoding(datadogV2.CONTENTENCODING_GZIP)

// NewSender can be used to create a new datadog api for sending logs to backend
func NewSender(endpoint string, logger *zap.Logger, s exporterhelper.TimeoutSettings, insecureSkipVerify bool, apiKey string) *Sender {
	cfg := datadog.NewConfiguration()
	logger.Info("using endpoint {}", zap.String("endpoint", endpoint))
	cfg.OperationServers[logsV2] = datadog.ServerConfigurations{
		datadog.ServerConfiguration{
			URL: endpoint,
		},
	}
	cfg.HTTPClient = utils.NewHTTPClient(s, insecureSkipVerify)
	cfg.AddDefaultHeader("DD-API-KEY", apiKey)
	apiClient := datadog.NewAPIClient(cfg)
	return &Sender{
		api:    datadogV2.NewLogsApi(apiClient),
		logger: logger,
	}
}

// AddLogItem would add the log to the buffer
func (s *Sender) AddLogItem(it datadogV2.HTTPLogItem) {
	s.buffer = append(s.buffer, it)

}

// Flush would send the buffered logs to the backend
func (s *Sender) Flush(ctx context.Context) {
	s.logger.Debug("submitting logs ", zap.String("logs", fmt.Sprintf("%v", s.buffer)))
	resp, r, err := s.api.SubmitLog(ctx, s.buffer, submitOpts)
	if err != nil {
		// TODO: should we add any retry mechanisms
		s.logger.Error("unable to send logs to datadog", zap.Error(err), zap.String("response", fmt.Sprintf("%v", r)))
		return
	}
	rc, _ := json.MarshalIndent(resp, "", "")
	s.logger.Debug("response from datadog", zap.String("response", string(rc)))
	s.buffer = nil

}
