// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

func TestScrapeSelf(t *testing.T) {
	s := SelfScraper{
		BlockProfileFraction: 1,
		MutexProfileFraction: 1,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()
	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	sp := p.ResourceProfiles().At(0).ScopeProfiles().At(0)

	require.Equal(
		t,
		metadata.ScopeName+"/selfscraper",
		sp.Scope().Name(),
	)

	require.Equal(t, "1.2.3", sp.Scope().Version())
}
