// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/hostmetadata"

import (
	"time"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"
)

// GetSourceProvider returns a provider which can be used to identify a source
func GetSourceProvider(set component.TelemetrySettings, configHostname string, timeout time.Duration) (source.Provider, error) {
	return hostmetadata.GetSourceProvider(set, configHostname, timeout)
}
