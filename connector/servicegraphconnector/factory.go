// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"
)

var NewFactory = servicegraphprocessor.NewConnectorFactoryFunc(metadata.Type, metadata.TracesToMetricsStability)
