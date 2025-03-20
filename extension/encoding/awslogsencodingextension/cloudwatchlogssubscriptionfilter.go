// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"
)

type cloudWatchLogsSubscriptionFilterUnmarshaler struct{}

func (cloudWatchLogsSubscriptionFilterUnmarshaler) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	// TODO implement this
	return plog.Logs{}, errors.New("not implemented")
}
