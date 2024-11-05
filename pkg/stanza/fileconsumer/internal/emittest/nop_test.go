// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/stretchr/testify/require"
)

func TestNop(t *testing.T) {
	require.NoError(t, Nop(context.Background(), emit.Token{}))
}
