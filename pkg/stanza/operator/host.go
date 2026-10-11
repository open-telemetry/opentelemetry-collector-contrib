// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import "go.opentelemetry.io/collector/component"

// HostSetter is an optional interface that operators can implement when they
// need access to the component.Host, for example to resolve extensions such as
// server authenticators. When an operator implements it, the host is provided
// before the operator is started.
type HostSetter interface {
	SetHost(component.Host)
}
