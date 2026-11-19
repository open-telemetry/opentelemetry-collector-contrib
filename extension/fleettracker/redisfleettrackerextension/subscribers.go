// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisfleettrackerextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension"

// fleetTracker mirrors the consumer-side contract the adaptive_tail_sampling
// processor asserts structurally via host.GetExtensions (contrib #50577).
type fleetTracker interface {
	SubscribeMemberCount(callback func(count int)) (cancel func(), err error)
}

var _ fleetTracker = (*fleetTrackerExtension)(nil)
