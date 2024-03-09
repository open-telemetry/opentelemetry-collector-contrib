// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver

import (
	"testing"

	"go.uber.org/goleak"
)

// Regarding the godbus/dbus ignore: see https://github.com/99designs/keyring/issues/103
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreAnyFunction("github.com/godbus/dbus.(*Conn).inWorker"))
}
