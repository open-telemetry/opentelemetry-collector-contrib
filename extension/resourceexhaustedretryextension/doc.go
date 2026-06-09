// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package resourceexhaustedretryextension implements a gRPC and HTTP server
// middleware that injects retry hints into RESOURCE_EXHAUSTED / 429 responses.
package resourceexhaustedretryextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/resourceexhaustedretryextension"
