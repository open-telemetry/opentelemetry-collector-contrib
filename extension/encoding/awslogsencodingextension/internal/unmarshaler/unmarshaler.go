// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler

import (
	"io"

	"go.opentelemetry.io/collector/pdata/plog"
)

type AWSUnmarshaler interface {
	UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error)
}
