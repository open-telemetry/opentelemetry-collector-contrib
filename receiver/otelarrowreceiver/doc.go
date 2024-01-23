// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package otelarrowexporter receives telemetry using OpenTelemetry
// Protocol with Apache Arrow and/or standard OpenTelemetry Protocol
// data using configuration structures similar to the core OTLP/gRPC
// receiver.
package otelarrowreceiver
