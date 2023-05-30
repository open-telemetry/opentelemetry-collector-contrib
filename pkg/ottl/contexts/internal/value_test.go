// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_SetIndexableValue_EmptyValueNoIndex(t *testing.T) {
	keys := []ottl.Key{
		{},
	}
	err := setIndexableValue(pcommon.NewValueEmpty(), nil, keys)
	assert.Error(t, err)
}
