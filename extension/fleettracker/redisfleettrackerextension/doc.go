// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package redisfleettrackerextension backs a fleet membership count with
// Redis, so collector instances that need to know how many peers are alive can
// subscribe to a live member count without running their own coordination
// protocol. The adaptive_tail_sampling processor uses it to divide an
// adaptive_throughput rule's goal_throughput across the fleet.
package redisfleettrackerextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension"

//go:generate make mdatagen
