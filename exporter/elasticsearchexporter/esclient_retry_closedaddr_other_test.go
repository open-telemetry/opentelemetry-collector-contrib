// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !unix && !windows

package elasticsearchexporter

import "testing"

func closedLocalAddr(t *testing.T) string {
	t.Helper()
	t.Skip("holding a closed TCP port requires a unix or windows socket")
	return ""
}
