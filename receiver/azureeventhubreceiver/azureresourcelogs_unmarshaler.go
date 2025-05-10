// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"
)

type logsUnmarshaler interface {
	UnmarshalLogs([]byte) (plog.Logs, error)
}

type AzureResourceLogsEventUnmarshaler struct {
	unmarshaler logsUnmarshaler
}

func newAzureResourceLogsUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger, applySemanticConventions bool) eventLogsUnmarshaler {
	if applySemanticConventions {
		return AzureResourceLogsEventUnmarshaler{
			unmarshaler: &azurelogs.ResourceLogsUnmarshaler{
				Version: buildInfo.Version,
				Logger:  logger,
			},
		}
	}
	return AzureResourceLogsEventUnmarshaler{
		unmarshaler: &azure.ResourceLogsUnmarshaler{
			Version: buildInfo.Version,
			Logger:  logger,
		},
	}
}

// UnmarshalLogs takes a byte array containing a JSON-encoded
// payload with Azure log records and transforms it into
// an OpenTelemetry plog.Logs object. The data in the Azure
// log record appears as fields and attributes in the
// OpenTelemetry representation; the bodies of the
// OpenTelemetry log records are empty.
func (r AzureResourceLogsEventUnmarshaler) UnmarshalLogs(event *eventhub.Event) (plog.Logs, error) {
	return r.unmarshaler.UnmarshalLogs(event.Data)
}
