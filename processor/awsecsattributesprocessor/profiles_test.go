// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func TestConsumeProfiles(t *testing.T) {
	srv := newMetadataServer(t)
	cfg := defaultTestConfig()
	require.NoError(t, cfg.init())
	p := newProfilesProcessor(zaptestLogger(t), cfg, consumertest.NewNop(), staticEndpoints(srv.URL))

	pd := pprofile.NewProfiles()
	pd.ResourceProfiles().AppendEmpty().Resource().Attributes().PutStr("container.id", testContainerID)

	require.NoError(t, p.ConsumeProfiles(t.Context(), pd))

	attrs := pd.ResourceProfiles().At(0).Resource().Attributes()
	// container.id source is overwritten by the enriched container.id.
	require.Equal(t, len(expectedFlattenedMetadata), attrs.Len())
	v, ok := attrs.Get("aws.ecs.cluster")
	require.True(t, ok)
	require.Equal(t, "cds-305", v.AsString())
}
