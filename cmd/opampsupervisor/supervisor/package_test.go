// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {

	goleak.VerifyTestMain(
		m,
		// This leaky goroutine it seems to come from using the cosign library.
		goleak.IgnoreAnyFunction("github.com/syndtr/goleveldb/leveldb.(*DB).mpoolDrain"),
	)
}
