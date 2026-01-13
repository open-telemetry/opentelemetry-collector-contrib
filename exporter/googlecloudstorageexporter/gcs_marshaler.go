// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type gcsMarshaler struct {
	logsMarshaler plog.Marshaler
	logger        *zap.Logger
	fileFormat    string
	IsCompressed  bool
}

func (marshaler *gcsMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	return marshaler.logsMarshaler.MarshalLogs(ld)
}

func (marshaler *gcsMarshaler) format() string {
	return marshaler.fileFormat
}

func (marshaler *gcsMarshaler) compressed() bool {
	return marshaler.IsCompressed
}
