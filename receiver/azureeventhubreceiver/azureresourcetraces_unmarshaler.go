// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure"
)

type azureTracesEventUnmarshaler struct {
	unmarshaler *azure.TracesUnmarshaler
}

func newAzureTracesUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger, timeFormat []string) eventTracesUnmarshaler {
	return azureTracesEventUnmarshaler{
		unmarshaler: &azure.TracesUnmarshaler{
			Version:     buildInfo.Version,
			Logger:      logger,
			TimeFormats: timeFormat,
		},
	}
}

// UnmarshalTraces takes a byte array containing a JSON-encoded
// payload with Azure records and transforms it into
// an OpenTelemetry ptraces.traces object. The data in the Azure
// record appears as fields and attributes in the
// OpenTelemetry representation; the bodies of the
// OpenTelemetry trace records are empty.
func (r azureTracesEventUnmarshaler) UnmarshalTraces(event *azureEvent) (ptrace.Traces, error) {
	return r.unmarshaler.UnmarshalTraces(event.Data())
}
