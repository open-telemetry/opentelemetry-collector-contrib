// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func newTestExtension(t *testing.T, cfg Config) *ext {
	extension := newExtension(&cfg)
	err := extension.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		err = extension.Shutdown(context.Background())
		require.NoError(t, err)
	})
	return extension
}
