// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_SetIndexableValue_EmptyValueNoIndex(t *testing.T) {
	keys := []ottl.Key[any]{
		&TestKey[any]{},
	}
	err := setIndexableValue[any](context.Background(), nil, pcommon.NewValueEmpty(), nil, keys)
	assert.Error(t, err)
}
