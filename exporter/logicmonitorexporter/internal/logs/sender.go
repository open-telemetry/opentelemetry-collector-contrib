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

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/logs"

import (
	"context"
	"fmt"
	"net/http"
	"time"

	lmsdklogs "github.com/logicmonitor/lm-data-sdk-go/api/logs"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

type Sender struct {
	logger          *zap.Logger
	logIngestClient *lmsdklogs.LMLogIngest
}

// NewSender creates a new Sender
func NewSender(ctx context.Context, endpoint string, client *http.Client, authParams utils.AuthParams, logger *zap.Logger) (*Sender, error) {
	options := []lmsdklogs.Option{
		lmsdklogs.WithLogBatchingDisabled(),
		lmsdklogs.WithAuthentication(authParams),
		lmsdklogs.WithHTTPClient(client),
		lmsdklogs.WithEndpoint(endpoint),
	}

	logIngestClient, err := lmsdklogs.NewLMLogIngest(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create logIngestClient: %w", err)
	}
	return &Sender{
		logger:          logger,
		logIngestClient: logIngestClient,
	}, nil
}

func (s *Sender) SendLogs(ctx context.Context, payload []model.LogInput) error {
	ingestResponse, err := s.logIngestClient.SendLogs(ctx, payload)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	if ingestResponse != nil {

		if ingestResponse.Success {
			return nil
		}
		if ingestResponse.RetryAfter > 0 {
			return exporterhelper.NewThrottleRetry(ingestResponse.Error, time.Duration(ingestResponse.RetryAfter)*time.Second)
		}
		if ingestResponse.StatusCode == http.StatusMultiStatus {
			for _, status := range ingestResponse.MultiStatus {
				if isPermanentClientFailure(int(status.Code)) {
					return consumererror.NewPermanent(fmt.Errorf("permanent failure error %s, complete error log %w", status.Error, ingestResponse.Error))
				}
			}
		}
		if isPermanentClientFailure(ingestResponse.StatusCode) {
			return consumererror.NewPermanent(ingestResponse.Error)
		}
		return ingestResponse.Error
	}
	return nil
}

// Does the 'code' indicate a permanent error
func isPermanentClientFailure(code int) bool {
	switch code {
	case http.StatusServiceUnavailable:
		return false
	case http.StatusGatewayTimeout:
		return false
	case http.StatusBadGateway:
		return false
	default:
		return true
	}
}
