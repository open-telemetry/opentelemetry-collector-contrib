// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sourceprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/sourceprovider"

import (
	"time"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata"
	"go.opentelemetry.io/collector/component"
)

func GetSourceProvider(set component.TelemetrySettings, configHostname string, timeout time.Duration) (source.Provider, error) {
	return hostmetadata.GetSourceProvider(set, configHostname, timeout)
}
