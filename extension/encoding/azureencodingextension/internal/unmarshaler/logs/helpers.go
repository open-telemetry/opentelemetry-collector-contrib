// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"go.opentelemetry.io/collector/pdata/plog"
)

// asSeverity converts the Azure log level to equivalent
// OpenTelemetry severity numbers. If the log level is not
// valid, then the 'Unspecified' value is returned.
// According to the documentation, the level Must be one of:
// `Informational`, `Warning`, `Error` or `Critical`.
// see https://learn.microsoft.com/en-us/azure/azure-monitor/platform/resource-logs-schema
func asSeverity(input string) plog.SeverityNumber {
	switch input {
	case "Informational":
		return plog.SeverityNumberInfo
	case "Warning":
		return plog.SeverityNumberWarn
	case "Error":
		return plog.SeverityNumberError
	case "Critical":
		return plog.SeverityNumberFatal
	default:
		return plog.SeverityNumberUnspecified
	}
}
