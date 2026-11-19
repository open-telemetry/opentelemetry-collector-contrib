// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package xcompression provides a simplified means of handling compression
// within the collector by providing a concurrency safe [Writer] and [Reader]
// type that enable a performance gain once used.
// For developer convenience as well, a [Nop] exists to remove any checks for if compression set
// within data processing paths.
package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"
