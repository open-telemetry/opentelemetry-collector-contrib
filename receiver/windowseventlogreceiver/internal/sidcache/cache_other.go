// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package sidcache

import "errors"

// lookupSID is a stub for non-Windows platforms
// This allows the code to compile on macOS/Linux during development
func lookupSID(_ string) (*ResolvedSID, error) {
	return nil, errors.New("SID lookup is only supported on Windows")
}
