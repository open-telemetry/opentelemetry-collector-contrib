// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package fips140test checks the Atlas digest authentication under GODEBUG=fips140=only.
// The //go:debug directive applies to the whole test binary, hence its own package.
package fips140test // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/fips140test"
