// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package supervisor // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"

import (
	"errors"
	"net"
)

// verifyPeerCredentials is not implemented on platforms without a supported
// peer-credential mechanism. Unix domain socket transport is rejected by config
// validation on those platforms (see Agent.Validate), so this is defensive.
func verifyPeerCredentials(_ *net.UnixConn, _, _ int) error {
	return errors.New("peer credential authentication is not supported on this platform")
}
