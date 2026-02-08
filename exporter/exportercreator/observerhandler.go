// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// observerHandler handles endpoint notifications from observers.
type observerHandler struct {
	// TODO: Implement in PR2
}

// Notify implements observer.Notify interface.
func (oh *observerHandler) OnAdd(added []observer.Endpoint) {
	// TODO: Implement in PR2
}

// OnRemove implements observer.Notify interface.
func (oh *observerHandler) OnRemove(removed []observer.Endpoint) {
	// TODO: Implement in PR2
}

// OnChange implements observer.Notify interface.
func (oh *observerHandler) OnChange(changed []observer.Endpoint) {
	// TODO: Implement in PR2
}

// shutdown stops all sub-exporters.
func (oh *observerHandler) shutdown() error {
	// TODO: Implement in PR2
	return nil
}
