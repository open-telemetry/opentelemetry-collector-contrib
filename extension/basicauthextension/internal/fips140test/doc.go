// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package fips140test runs the authenticator under GODEBUG=fips140=only, which
// the //go:debug directive sets for the whole test binary.
package fips140test // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension/internal/fips140test"
