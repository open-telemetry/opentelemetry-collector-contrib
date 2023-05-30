// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/traces"

import (
	"context"
	"fmt"
	"net/http"
	"time"

	lmsdktraces "github.com/logicmonitor/lm-data-sdk-go/api/traces"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type Sender struct {
	logger            *zap.Logger
	traceIngestClient *lmsdktraces.LMTraceIngest
}

// NewSender creates a new Sender
func NewSender(ctx context.Context, endpoint string, client *http.Client, authParams utils.AuthParams, logger *zap.Logger) (*Sender, error) {
	options := []lmsdktraces.Option{
		lmsdktraces.WithTraceBatchingDisabled(),
		lmsdktraces.WithAuthentication(authParams),
		lmsdktraces.WithHTTPClient(client),
		lmsdktraces.WithEndpoint(endpoint),
	}

	traceIngestClient, err := lmsdktraces.NewLMTraceIngest(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create traceIngestClient: %w", err)
	}
	return &Sender{
		logger:            logger,
		traceIngestClient: traceIngestClient,
	}, nil
}

func (s *Sender) SendTraces(ctx context.Context, td ptrace.Traces) error {
	ingestResponse, err := s.traceIngestClient.SendTraces(ctx, td)
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
