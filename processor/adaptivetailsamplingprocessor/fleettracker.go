// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// FleetTracker reports the live number of collector instances in this
// collector's fleet. The processor resolves the extension named by the
// fleet_tracker setting against this interface structurally via
// host.GetExtensions, so extension implementations do not need to import
// this package; the method signature is the contract.
type FleetTracker interface {
	// SubscribeMemberCount registers callback to receive the fleet's live
	// member count. The callback is invoked once with the current count when
	// the subscription is established, and again on every change. Callbacks
	// must not block. The returned cancel func unregisters the callback and
	// must be safe to call once.
	SubscribeMemberCount(callback func(count int)) (cancel func(), err error)
}

// resolveFleetTracker resolves the extension named by id against host, and
// asserts it implements FleetTracker.
func resolveFleetTracker(host component.Host, id component.ID) (FleetTracker, error) {
	if host == nil {
		return nil, errors.New("fleet_tracker configured but host is nil")
	}
	extension, ok := host.GetExtensions()[id]
	if !ok {
		return nil, fmt.Errorf("fleet_tracker extension %q not found", id)
	}
	tracker, ok := extension.(FleetTracker)
	if !ok {
		return nil, fmt.Errorf("extension %q does not implement FleetTracker", id)
	}
	return tracker, nil
}
