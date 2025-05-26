// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

func Test_SetIndexableValue_InvalidValue(t *testing.T) {
	keys := []ottl.Key[any]{
		&pathtest.Key[any]{},
	}
	err := ctxutil.SetIndexableValue[any](context.Background(), nil, pcommon.NewValueStr("str"), nil, keys)
	assert.Error(t, err)
}
