// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/apmstats"
)

const nativeIngestFeatureGateName = "connector.datadogconnector.NativeIngest"

// NativeIngestFeatureGate is the feature gate that controls native OTel spans ingestion in Datadog APM stats
var NativeIngestFeatureGate = featuregate.GlobalRegistry().MustRegister(
	nativeIngestFeatureGateName,
	featuregate.StageStable,
	featuregate.WithRegisterDescription("When enabled, datadogconnector uses the native OTel API to ingest OTel spans and produce APM stats."),
	featuregate.WithRegisterFromVersion("v0.104.0"),
	featuregate.WithRegisterToVersion("v0.143.0"),
)

// NewFactory creates a factory for datadog connector.
func NewFactory() connector.Factory {
	//  OTel connector factory to make a factory for connectors
	return apmstats.NewConnectorFactory(metadata.Type, metadata.TracesToMetricsStability, metadata.TracesToTracesStability, nil, nil, nil)
}
