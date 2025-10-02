// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !debug

package nfsscraper

func debugDump(prefix, path string) {
	// This is a no-op function that will be compiled in when the 'debug' build tag is not used.
}
