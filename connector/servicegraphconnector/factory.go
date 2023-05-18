// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"

var NewFactory = servicegraphprocessor.NewConnectorFactory
