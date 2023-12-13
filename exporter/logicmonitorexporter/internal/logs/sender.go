// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/logs"

import (
	"context"
	"fmt"
	"net/http"
	"time"

	lmsdklogs "github.com/logicmonitor/lm-data-sdk-go/api/logs"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

type Sender struct {
	logger          *zap.Logger
	logIngestClient *lmsdklogs.LMLogIngest
}

// NewSender creates a new Sender
func NewSender(ctx context.Context, logger *zap.Logger, opts ...lmsdklogs.Option) (*Sender, error) {
	logIngestClient, err := lmsdklogs.NewLMLogIngest(ctx, opts...)
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
