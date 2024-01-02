// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/generate"
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestNodeDOTID(t *testing.T) {
	operator := testutil.NewMockOperator("test")
	operator.On("Outputs").Return(nil)
	node := createOperatorNode(operator)
	require.Equal(t, operator.ID(), node.DOTID())
}

func TestCreateNodeID(t *testing.T) {
	nodeID := createNodeID("test_id")
	require.Equal(t, int64(5795108767401590291), nodeID)
}
