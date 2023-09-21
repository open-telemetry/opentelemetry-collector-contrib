// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/correlations/diagnostics.go

package correlations // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"

import (
	"github.com/signalfx/golib/v3/datapoint"
	"github.com/signalfx/golib/v3/sfxclient"
)

// InternalMetrics returns datapoints that describe the current state of the
// dimension update client
func (cc *Client) InternalMetrics() []*datapoint.Datapoint {
	dps := []*datapoint.Datapoint{
		sfxclient.CumulativeP("sfxagent.correlation_updates_invalid", nil, &cc.TotalInvalidDimensions),
		// All 4xx HTTP responses that are not retried except 404 (which is retried)
		sfxclient.CumulativeP("sfxagent.correlation_updates_client_errors", nil, &cc.TotalClientError4xxResponses),
		sfxclient.CumulativeP("sfxagent.correlation_updates_retries", nil, &cc.TotalRetriedUpdates),
	}
	return append(dps, cc.requestSender.InternalMetrics()...)
}
