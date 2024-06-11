// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate go run go.opentelemetry.io/collector/cmd/mdatagen metadata.yaml

// Package opampsupervisor is an implementation of an Open Agent Management Protocol (OpAMP)
// 'supervisor' - which is the process that manages local execution of the OpenTelemetry Collector
// as a local agent in hosts or containers.
package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor
