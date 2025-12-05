// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Ignore ttlcache goroutines - cache.Start() may not return immediately after Stop()
		goleak.IgnoreTopFunction("github.com/jellydator/ttlcache/v3.(*Cache[...]).Start"),
	)
}
